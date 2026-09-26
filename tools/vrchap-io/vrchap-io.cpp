// vrchap-io: export / import the *live* SteamVR chaperone (play area) through the official OpenVR API,
// the same way Room Setup does it (IVRChaperoneSetup::ExportLiveToBuffer / ImportFromBufferToWorking).
//
//   vrchap-io check              -> exit 0 if the chaperone-setup interface is reachable
//   vrchap-io export             -> prints the live chaperone JSON to stdout
//   vrchap-io import <file|->    -> imports JSON (file or stdin) into the working copy and commits it as live
//                                   add --bounds-only to import only the collision bounds
//
// SteamVR must already be running: this helper refuses to start it (exit code 3).
// Env VRZONES_APPTYPE = background | overlay | utility (default: overlay).
#include <openvr.h>
#include <dirent.h>

#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <fstream>
#include <iostream>
#include <sstream>
#include <string>
#include <vector>

using namespace vr;

static bool vrserver_running() {
    DIR *d = opendir("/proc");
    if (!d) return false;
    bool found = false;
    while (dirent *e = readdir(d)) {
        if (e->d_name[0] < '0' || e->d_name[0] > '9') continue;
        std::string path = std::string("/proc/") + e->d_name + "/comm";
        std::ifstream f(path);
        std::string comm;
        if (f && std::getline(f, comm) && comm == "vrserver") { found = true; break; }
    }
    closedir(d);
    return found;
}

static EVRApplicationType app_type() {
    const char *t = getenv("VRZONES_APPTYPE");
    if (t && !strcmp(t, "background")) return VRApplication_Background;
    if (t && !strcmp(t, "utility")) return VRApplication_Utility;
    return VRApplication_Overlay;
}

int main(int argc, char **argv) {
    if (argc < 2) {
        fprintf(stderr, "usage: vrchap-io check | export | import <file|-> [--bounds-only]\n");
        return 2;
    }
    std::string cmd = argv[1];

    if (!vrserver_running()) {
        fprintf(stderr, "SteamVR is not running (vrserver not found)\n");
        return 3;
    }

    EVRInitError err;
    VR_Init(&err, app_type());
    if (err != VRInitError_None) {
        fprintf(stderr, "VR_Init failed: %s\n", VR_GetVRInitErrorAsEnglishDescription(err));
        return 3;
    }
    IVRChaperoneSetup *cs = VRChaperoneSetup();
    if (!cs) {
        fprintf(stderr, "IVRChaperoneSetup is not available for this app type\n");
        VR_Shutdown();
        return 4;
    }

    int rc = 0;
    if (cmd == "check") {
        printf("ok\n");
    } else if (cmd == "state") {
        IVRChaperone *ch = VRChaperone();
        if (!ch) { fprintf(stderr, "IVRChaperone is not available\n"); rc = 4; }
        else {
            int s = (int)ch->GetCalibrationState();
            const char *name = "unknown";
            switch (s) {
                case 1: name = "OK"; break;
                case 100: name = "Warning"; break;
                case 101: name = "Warning_BaseStationMayHaveMoved"; break;
                case 102: name = "Warning_BaseStationRemoved"; break;
                case 103: name = "Warning_SeatedBoundsInvalid"; break;
                case 200: name = "Error (UniverseID invalid)"; break;
                case 201: name = "Error_BaseStationUninitialized"; break;
                case 202: name = "Error_BaseStationConflict"; break;
                case 203: name = "Error_PlayAreaInvalid"; break;
                case 204: name = "Error_CollisionBoundsInvalid"; break;
            }
            printf("%d %s\n", s, name);
        }
    } else if (cmd == "export") {
        uint32_t len = 0;
        cs->ExportLiveToBuffer(nullptr, &len);
        if (len < 4096) len = 1 << 20;
        std::vector<char> buf(len + 1, 0);
        uint32_t cap = len;
        if (!cs->ExportLiveToBuffer(buf.data(), &cap)) {
            fprintf(stderr, "ExportLiveToBuffer failed\n");
            rc = 4;
        } else {
            fputs(buf.data(), stdout);
            fputc('\n', stdout);
        }
    } else if (cmd == "import") {
        if (argc < 3) {
            fprintf(stderr, "import needs a file or - for stdin\n");
            rc = 2;
        } else {
            std::stringstream ss;
            if (!strcmp(argv[2], "-")) ss << std::cin.rdbuf();
            else {
                std::ifstream f(argv[2]);
                if (!f) { fprintf(stderr, "cannot open %s\n", argv[2]); VR_Shutdown(); return 2; }
                ss << f.rdbuf();
            }
            std::string json = ss.str();
            uint32_t flags = 0;
            for (int i = 3; i < argc; i++)
                if (!strcmp(argv[i], "--bounds-only")) flags |= EChaperoneImport_BoundsOnly;
            if (!cs->ImportFromBufferToWorking(json.c_str(), flags)) {
                fprintf(stderr, "ImportFromBufferToWorking failed (bad JSON or wrong universe?)\n");
                rc = 4;
            } else if (!cs->CommitWorkingCopy(EChaperoneConfigFile_Live)) {
                fprintf(stderr, "CommitWorkingCopy failed\n");
                rc = 4;
            } else {
                printf("applied\n");
            }
        }
    } else {
        fprintf(stderr, "unknown command: %s\n", cmd.c_str());
        rc = 2;
    }
    VR_Shutdown();
    return rc;
}
