import { Power, PowerOff, SettingsIcon, X, XIcon } from "lucide-preact";
import { ChangeBaseStationPowerStatus, UpdateConfigValue } from "@src/lib/native/index";
import { useContext, useEffect, useState } from "preact/hooks";
import { AnimatePresence, motion } from 'framer-motion';
import { PowerStatusIcon } from "../assets/icons/PowerStatusIcon";
import { TitleBarSettingsIcon } from "../assets/icons/TitleBarSettingsIcon";
import { CloseIcon } from "../assets/icons/CloseIcon";
import { route } from "preact-router";
import { useTranslation } from "react-i18next";
import { useLighthouses } from "@src/lib/hooks/useLighthouses";
import { useConfig } from "@src/lib/hooks/useConfig";
import { useSteamVRStatus } from "@src/lib/hooks/useSteamVRStatus";
import { WebsocketContext } from "@src/lib/context/websocket.context";
import { usePlatform } from "@src/lib/hooks/usePlatform";
import { useLighthouseGroups } from "@src/lib/hooks/useLighthouseGroups";



export function TitleBar() {

    const { websocket, send } = useContext(WebsocketContext);
    const steamVRLaunched = useSteamVRStatus();
    const platform = usePlatform();
    const groups = useLighthouseGroups();
    const [previousSteamVRState, setPreviousSteamVRState] = useState(false);
    const lighthouses = useLighthouses();
    const config = useConfig();

    const { t } = useTranslation();

    // Returns the stations that refused the command rather than reporting them
    // itself: the manual buttons surface these, while the SteamVR automation
    // below only logs them - it runs unattended, often with the window hidden,
    // where a dialog would be stuck behind the tray.
    const bulkUpdate = async (state: "sleep" | "awake", flags: number = 0) => {
        const failures: string[] = [];

        for(const baseStation of lighthouses) {
            if (flags && !((baseStation.managed_flags & flags) > 0)) continue;
            const result = await ChangeBaseStationPowerStatus(baseStation.id, state);
            if (result != "ok") failures.push(`${baseStation.name}: ${result}`);
        }

        return failures;
    }

    useEffect(() => {
        (async () => {
            if (!config) return;
            if (!config.is_steamvr_managed) return;

            // Only react to an actual SteamVR transition. Without this the
            // effect also fires on mount and puts the base stations to sleep
            // right after the app starts.
            if (steamVRLaunched === previousSteamVRState) return;

            setPreviousSteamVRState(steamVRLaunched);

            if (steamVRLaunched) {
                console.log("Waking up")
                const failures = await bulkUpdate("awake", 2);
                if (failures.length) console.error("Failed to wake:", failures);
                return;
            }

            console.log("Putting in sleep mode")
            const failures = await bulkUpdate("sleep", 4);
            if (failures.length) console.error("Failed to sleep:", failures);
        })()
    }, [steamVRLaunched]);

  
    const [powerBusy, setPowerBusy] = useState(false);

    // Explicit commands rather than one toggle - the app can't read a station's
    // power state on Windows, so a toggle would have to guess. See BaseStation.
    //
    // Note: this deliberately doesn't touch previousSteamVRState. That tracks
    // SteamVR only, and writing to it here made a manual press flip the
    // automation so the next SteamVR launch sent "sleep" instead of "awake".
    const setAllPower = async (mode: "awake" | "sleep") => {
        // Held only while the commands are in flight - see BaseStation.
        if (powerBusy) return;
        setPowerBusy(true);

        try {
            const failures = await bulkUpdate(mode);
            if (failures.length) alert(failures.join("\n"));
        } finally {
            setPowerBusy(false);
        }
    }

    const Quit = async () => {


        if (config.allow_tray) {
            // HideToTray also releases the base stations, so they stay usable
            // from another machine while we sit in the tray.
            //@ts-ignore
            await window.go.main.App.HideToTray()

            if (!config.tray_notified) {
                //@ts-ignore
                await window.go.main.App.Notify("SteamVR Lighthosue Manager", t("Window was hidden in the tray."))
                await UpdateConfigValue("tray_notified", true)
            }

            return
        }
        
        //@ts-ignore
        return await window.runtime.Quit()
    }

    return <div className="flex flex-row justify-between pt-[16px] px-[24px] select-none" style="--wails-draggable:drag">
        <div className="flex gap-1 flex-row">
            <div className="text-[#888888] poppins-medium text-[14px]/[20px]">
                SteamVR Lighthouse Manager <span className={"text-[#505050] poppins-medium text-[12px]/[20px]"}>{platform.version}</span>
            </div>
        </div>
        <div>

        </div>
        <div className="flex flex-row gap-1 items-center" style={"--wails-draggable:no-drag"}>
            <AnimatePresence>
                {config.is_steamvr_installed && config && config.is_steamvr_managed && <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} ><div className="flex flex-row gap-[4px] text-white poppins-regular text-[14px] items-center px-2">
                    <span className="text-[##C6C6C6]">SteamVR</span>
                    <span className={`data-[active="true"]:text-[#7AFF73] text-[#FF7373] duration-200`} data-active={steamVRLaunched}>{steamVRLaunched ? t("Active") : t("Inactive")} </span>
                </div></motion.div>}

                {!websocket.ready && <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} ><div className="poppins-regular text-sm items-center px-2">
                    <span className={`text-[#FF7373] bg-[#FF7373]/20 rounded-xl border-[#FF7373] border-[0.5px] p-1 text-sm`}>Server connection failed.</span>
                </div></motion.div>}


            </AnimatePresence>
            <button className="opacity-75 hover:opacity-100 duration-150 disabled:opacity-25" onClick={() => setAllPower("awake")} disabled={powerBusy} title={t("Turn on")}>
                <Power color="#C6C6C6"/>
            </button>
            <button className="opacity-75 hover:opacity-100 duration-150 disabled:opacity-25" onClick={() => setAllPower("sleep")} disabled={powerBusy} title={t("Turn off")}>
                <PowerOff color="#C6C6C6"/>
            </button>
            <button onClick={(c) => route("/settings", true)}>
                {/* <TitleBarSettingsIcon width={16} height={16} fill="#888888" className={`hover:fill-[#1D81FF] duration-200`} /> */}
                <SettingsIcon color="#888888" />
            </button>
            <button onClick={() => Quit()}>
                {/* <CloseIcon  width={12} height={12} fill="#888888" className={`hover:fill-[#1D81FF] duration-200`} /> */}
                <X color="#888888" />
            </button>
        </div>
    </div>
}