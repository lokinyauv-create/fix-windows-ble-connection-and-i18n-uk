// VK_LAYER_vrfix_mipclamp
//
// SteamVR Room Setup (Unity 5.6) calls vkCreateImage with more mip levels than the Vulkan spec allows
// (e.g. 50x32 with mipLevels=7, max valid is floor(log2(50))+1 = 6). Mesa's RADV (this build, 26.2.2
// freeworld) then hits an assertion in addrlib (Addr2::V2::Lib::ComputeSurfaceInfo) and raises SIGTRAP.
//
// This layer clamps mipLevels in vkCreateImage to the valid maximum, and clamps the mip range of
// vkCreateImageView for the images it changed. Anything it touches is logged to stderr once per size.
//
// Build: see build.sh. Enable per process with the environment (see vr-room-setup).

#define VK_NO_PROTOTYPES
#include <vulkan/vulkan.h>
#include <vulkan/vk_layer.h>

#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define EXPORT __attribute__((visibility("default")))

typedef struct {
    void *key;
    PFN_vkGetInstanceProcAddr gipa;
    PFN_vkDestroyInstance destroy;
} InstData;

typedef struct {
    void *key;
    PFN_vkGetDeviceProcAddr gdpa;
    PFN_vkDestroyDevice destroy;
    PFN_vkCreateImage create_image;
    PFN_vkCreateImageView create_view;
} DevData;

typedef struct {
    VkImage image;
    uint32_t mips;
} Clamped;

#define MAX_INST 16
#define MAX_DEV 16
#define MAX_CLAMPED 4096

static InstData g_inst[MAX_INST];
static int g_ninst;
static DevData g_dev[MAX_DEV];
static int g_ndev;
static Clamped g_clamped[MAX_CLAMPED];
static int g_nclamped;
static pthread_mutex_t g_lock = PTHREAD_MUTEX_INITIALIZER;

#define KEY(obj) (*(void **)(obj))

static InstData *find_inst(void *handle) {
    InstData *r = NULL;
    if (!handle) return NULL;
    pthread_mutex_lock(&g_lock);
    for (int i = 0; i < g_ninst; i++)
        if (g_inst[i].key == KEY(handle)) { r = &g_inst[i]; break; }
    pthread_mutex_unlock(&g_lock);
    return r;
}

static DevData *find_dev(void *handle) {
    DevData *r = NULL;
    if (!handle) return NULL;
    pthread_mutex_lock(&g_lock);
    for (int i = 0; i < g_ndev; i++)
        if (g_dev[i].key == KEY(handle)) { r = &g_dev[i]; break; }
    pthread_mutex_unlock(&g_lock);
    return r;
}

static uint32_t max_valid_levels(const VkImageCreateInfo *ci) {
    uint32_t m = ci->extent.width;
    if (ci->extent.height > m) m = ci->extent.height;
    if (ci->extent.depth > m) m = ci->extent.depth;
    uint32_t n = 1;
    while (m > 1) { m >>= 1; n++; }
    return n;
}

static void remember(VkImage image, uint32_t mips) {
    pthread_mutex_lock(&g_lock);
    if (g_nclamped < MAX_CLAMPED) {
        g_clamped[g_nclamped].image = image;
        g_clamped[g_nclamped].mips = mips;
        g_nclamped++;
    }
    pthread_mutex_unlock(&g_lock);
}

static uint32_t lookup(VkImage image) {
    uint32_t r = 0;
    pthread_mutex_lock(&g_lock);
    for (int i = 0; i < g_nclamped; i++)
        if (g_clamped[i].image == image) { r = g_clamped[i].mips; break; }
    pthread_mutex_unlock(&g_lock);
    return r;
}

static void log_once(const char *what, uint32_t a, uint32_t b, uint32_t c, uint32_t d) {
    static struct { char w[24]; uint32_t v[4]; } seen[64];
    static int n;
    pthread_mutex_lock(&g_lock);
    for (int i = 0; i < n; i++)
        if (!strcmp(seen[i].w, what) && seen[i].v[0] == a && seen[i].v[1] == b && seen[i].v[2] == c && seen[i].v[3] == d) {
            pthread_mutex_unlock(&g_lock);
            return;
        }
    if (n < 64) {
        strncpy(seen[n].w, what, 23);
        seen[n].v[0] = a; seen[n].v[1] = b; seen[n].v[2] = c; seen[n].v[3] = d;
        n++;
    }
    pthread_mutex_unlock(&g_lock);
    fprintf(stderr, "[mipclamp] %s: %u %u %u %u\n", what, a, b, c, d);
}

// ---- hooks ----------------------------------------------------------------------------------

static VKAPI_ATTR VkResult VKAPI_CALL hook_CreateImage(VkDevice device, const VkImageCreateInfo *pCI,
                                                        const VkAllocationCallbacks *pA, VkImage *pImage) {
    DevData *d = find_dev(device);
    VkImageCreateInfo ci = *pCI;
    uint32_t maxl = max_valid_levels(pCI);
    int clamped = 0;
    if (ci.mipLevels > maxl) {
        log_once("vkCreateImage extent/mips->max", ci.extent.width, ci.extent.height, ci.mipLevels, maxl);
        ci.mipLevels = maxl;
        clamped = 1;
    }
    VkResult r = d->create_image(device, &ci, pA, pImage);
    if (r == VK_SUCCESS && clamped) remember(*pImage, ci.mipLevels);
    return r;
}

static VKAPI_ATTR VkResult VKAPI_CALL hook_CreateImageView(VkDevice device, const VkImageViewCreateInfo *pCI,
                                                            const VkAllocationCallbacks *pA, VkImageView *pView) {
    DevData *d = find_dev(device);
    VkImageViewCreateInfo ci = *pCI;
    uint32_t mips = lookup(ci.image);
    if (mips && ci.subresourceRange.levelCount != VK_REMAINING_MIP_LEVELS &&
        ci.subresourceRange.baseMipLevel + ci.subresourceRange.levelCount > mips) {
        log_once("vkCreateImageView base/count->mips", ci.subresourceRange.baseMipLevel, ci.subresourceRange.levelCount, mips, 0);
        if (ci.subresourceRange.baseMipLevel >= mips) ci.subresourceRange.baseMipLevel = mips - 1;
        ci.subresourceRange.levelCount = mips - ci.subresourceRange.baseMipLevel;
    }
    return d->create_view(device, &ci, pA, pView);
}

static VKAPI_ATTR void VKAPI_CALL hook_DestroyDevice(VkDevice device, const VkAllocationCallbacks *pA) {
    DevData *d = find_dev(device);
    PFN_vkDestroyDevice destroy = d->destroy;
    pthread_mutex_lock(&g_lock);
    for (int i = 0; i < g_ndev; i++)
        if (&g_dev[i] == d) { g_dev[i] = g_dev[--g_ndev]; break; }
    pthread_mutex_unlock(&g_lock);
    destroy(device, pA);
}

static VKAPI_ATTR void VKAPI_CALL hook_DestroyInstance(VkInstance instance, const VkAllocationCallbacks *pA) {
    InstData *i = find_inst(instance);
    PFN_vkDestroyInstance destroy = i->destroy;
    pthread_mutex_lock(&g_lock);
    for (int k = 0; k < g_ninst; k++)
        if (&g_inst[k] == i) { g_inst[k] = g_inst[--g_ninst]; break; }
    pthread_mutex_unlock(&g_lock);
    destroy(instance, pA);
}

static VKAPI_ATTR VkResult VKAPI_CALL hook_CreateInstance(const VkInstanceCreateInfo *pCI,
                                                           const VkAllocationCallbacks *pA, VkInstance *pInstance) {
    VkLayerInstanceCreateInfo *chain = (VkLayerInstanceCreateInfo *)pCI->pNext;
    while (chain && !(chain->sType == VK_STRUCTURE_TYPE_LOADER_INSTANCE_CREATE_INFO && chain->function == VK_LAYER_LINK_INFO))
        chain = (VkLayerInstanceCreateInfo *)chain->pNext;
    if (!chain) return VK_ERROR_INITIALIZATION_FAILED;

    PFN_vkGetInstanceProcAddr next_gipa = chain->u.pLayerInfo->pfnNextGetInstanceProcAddr;
    chain->u.pLayerInfo = chain->u.pLayerInfo->pNext;
    PFN_vkCreateInstance create = (PFN_vkCreateInstance)next_gipa(VK_NULL_HANDLE, "vkCreateInstance");
    VkResult r = create(pCI, pA, pInstance);
    if (r != VK_SUCCESS) return r;

    pthread_mutex_lock(&g_lock);
    if (g_ninst < MAX_INST) {
        g_inst[g_ninst].key = KEY(*pInstance);
        g_inst[g_ninst].gipa = next_gipa;
        g_inst[g_ninst].destroy = (PFN_vkDestroyInstance)next_gipa(*pInstance, "vkDestroyInstance");
        g_ninst++;
    }
    pthread_mutex_unlock(&g_lock);
    return VK_SUCCESS;
}

static VKAPI_ATTR VkResult VKAPI_CALL hook_CreateDevice(VkPhysicalDevice pd, const VkDeviceCreateInfo *pCI,
                                                         const VkAllocationCallbacks *pA, VkDevice *pDevice) {
    VkLayerDeviceCreateInfo *chain = (VkLayerDeviceCreateInfo *)pCI->pNext;
    while (chain && !(chain->sType == VK_STRUCTURE_TYPE_LOADER_DEVICE_CREATE_INFO && chain->function == VK_LAYER_LINK_INFO))
        chain = (VkLayerDeviceCreateInfo *)chain->pNext;
    if (!chain) return VK_ERROR_INITIALIZATION_FAILED;

    PFN_vkGetInstanceProcAddr next_gipa = chain->u.pLayerInfo->pfnNextGetInstanceProcAddr;
    PFN_vkGetDeviceProcAddr next_gdpa = chain->u.pLayerInfo->pfnNextGetDeviceProcAddr;
    chain->u.pLayerInfo = chain->u.pLayerInfo->pNext;
    PFN_vkCreateDevice create = (PFN_vkCreateDevice)next_gipa(VK_NULL_HANDLE, "vkCreateDevice");
    VkResult r = create(pd, pCI, pA, pDevice);
    if (r != VK_SUCCESS) return r;

    pthread_mutex_lock(&g_lock);
    if (g_ndev < MAX_DEV) {
        g_dev[g_ndev].key = KEY(*pDevice);
        g_dev[g_ndev].gdpa = next_gdpa;
        g_dev[g_ndev].destroy = (PFN_vkDestroyDevice)next_gdpa(*pDevice, "vkDestroyDevice");
        g_dev[g_ndev].create_image = (PFN_vkCreateImage)next_gdpa(*pDevice, "vkCreateImage");
        g_dev[g_ndev].create_view = (PFN_vkCreateImageView)next_gdpa(*pDevice, "vkCreateImageView");
        g_ndev++;
    }
    pthread_mutex_unlock(&g_lock);
    return VK_SUCCESS;
}

// ---- proc addr plumbing ---------------------------------------------------------------------

// Some clients (SteamVR's vrclient.so) fetch VK_EXT_debug_utils entry points through vkGetInstanceProcAddr
// and call them without a NULL check. The loader normally provides them; when they reach us and the next
// layer has none, hand back harmless no-op stubs instead of NULL.
static VKAPI_ATTR VkResult VKAPI_CALL stub_result(void) { return VK_SUCCESS; }
static VKAPI_ATTR void VKAPI_CALL stub_void(void) {}

static PFN_vkVoidFunction debug_utils_stub(const char *name) {
    if (!strncmp(name, "vkSetDebugUtilsObject", 21)) return (PFN_vkVoidFunction)stub_result;
    if (strstr(name, "DebugUtilsLabelEXT")) return (PFN_vkVoidFunction)stub_void;
    return NULL;
}

static void log_null(const char *who, const char *name) {
    static char seen[128][72];
    static int n;
    static int debug = -1;
    if (debug < 0) debug = getenv("MIPCLAMP_DEBUG") != NULL;
    if (!debug) return;
    pthread_mutex_lock(&g_lock);
    for (int i = 0; i < n; i++)
        if (!strncmp(seen[i], name, 71)) { pthread_mutex_unlock(&g_lock); return; }
    if (n < 128) { strncpy(seen[n], name, 71); n++; }
    pthread_mutex_unlock(&g_lock);
    fprintf(stderr, "[mipclamp] %s returned NULL for %s\n", who, name);
}

// MIPCLAMP_TRACE=1: log every proc-addr lookup for names matching the substring in MIPCLAMP_TRACE_FILTER
static void trace(const char *who, const char *name, PFN_vkVoidFunction f) {
    static int on = -1;
    static const char *filter;
    if (on < 0) { on = getenv("MIPCLAMP_TRACE") != NULL; filter = getenv("MIPCLAMP_TRACE_FILTER"); }
    if (!on) return;
    if (filter && !strstr(name, filter)) return;
    fprintf(stderr, "[mipclamp] %s(%s) -> %p\n", who, name, (void *)f);
}

static VKAPI_ATTR PFN_vkVoidFunction VKAPI_CALL layer_GetDeviceProcAddr(VkDevice device, const char *name);

static VKAPI_ATTR PFN_vkVoidFunction VKAPI_CALL layer_GetInstanceProcAddr(VkInstance instance, const char *name) {
    if (!strcmp(name, "vkCreateInstance")) return (PFN_vkVoidFunction)hook_CreateInstance;
    if (!strcmp(name, "vkDestroyInstance")) return (PFN_vkVoidFunction)hook_DestroyInstance;
    if (!strcmp(name, "vkCreateDevice")) return (PFN_vkVoidFunction)hook_CreateDevice;
    if (!strcmp(name, "vkGetInstanceProcAddr")) return (PFN_vkVoidFunction)layer_GetInstanceProcAddr;
    if (!strcmp(name, "vkGetDeviceProcAddr")) return (PFN_vkVoidFunction)layer_GetDeviceProcAddr;
    if (!strcmp(name, "vkDestroyDevice")) return (PFN_vkVoidFunction)hook_DestroyDevice;
    if (!strcmp(name, "vkCreateImage")) return (PFN_vkVoidFunction)hook_CreateImage;
    if (!strcmp(name, "vkCreateImageView")) return (PFN_vkVoidFunction)hook_CreateImageView;
    if (!instance) { log_null("GIPA(instance=NULL)", name); return NULL; }
    InstData *i = find_inst(instance);
    if (!i) { log_null("GIPA(unknown instance)", name); return NULL; }
    PFN_vkVoidFunction f = i->gipa(instance, name);
    if (!f) f = debug_utils_stub(name);
    if (!f) log_null("GIPA(next)", name);
    trace("GIPA", name, f);
    return f;
}

static VKAPI_ATTR PFN_vkVoidFunction VKAPI_CALL layer_GetDeviceProcAddr(VkDevice device, const char *name) {
    if (!strcmp(name, "vkGetDeviceProcAddr")) return (PFN_vkVoidFunction)layer_GetDeviceProcAddr;
    if (!strcmp(name, "vkDestroyDevice")) return (PFN_vkVoidFunction)hook_DestroyDevice;
    if (!strcmp(name, "vkCreateImage")) return (PFN_vkVoidFunction)hook_CreateImage;
    if (!strcmp(name, "vkCreateImageView")) return (PFN_vkVoidFunction)hook_CreateImageView;
    DevData *d = find_dev(device);
    if (!d) { log_null("GDPA(unknown device)", name); return NULL; }
    PFN_vkVoidFunction f = d->gdpa(device, name);
    if (!f) f = debug_utils_stub(name);
    if (!f) log_null("GDPA(next)", name);
    trace("GDPA", name, f);
    return f;
}

EXPORT VKAPI_ATTR VkResult VKAPI_CALL vkNegotiateLoaderLayerInterfaceVersion(VkNegotiateLayerInterface *p) {
    if (!p || p->sType != LAYER_NEGOTIATE_INTERFACE_STRUCT) return VK_ERROR_INITIALIZATION_FAILED;
    if (p->loaderLayerInterfaceVersion >= 2) {
        p->loaderLayerInterfaceVersion = 2;
        p->pfnGetInstanceProcAddr = layer_GetInstanceProcAddr;
        p->pfnGetDeviceProcAddr = layer_GetDeviceProcAddr;
        p->pfnGetPhysicalDeviceProcAddr = NULL;
    }
    return VK_SUCCESS;
}
