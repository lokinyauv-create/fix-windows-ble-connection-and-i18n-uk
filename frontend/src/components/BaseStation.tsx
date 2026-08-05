import { BaseStationIcon } from "../assets/basestation";
import { motion, AnimatePresence } from "framer-motion"

import { useMemo, useState } from "preact/hooks";
import { ChangeBaseStationPowerStatus, IdentitifyBaseStation } from "@src/lib/native/index";
import { StatusCircleIcon } from "../assets/icons/StatusCircleIcon";
import { route } from "preact-router";
import { useTranslation } from "react-i18next";
import { ChevronRightIcon, Eye, Power, PowerOff } from "lucide-preact";
import type { LighthouseStation } from "@src/lib/types/index";
import { useWebsocketCommunication } from "@src/lib/hooks/useWebsocketCommunication";
import { isAwake, isUnknown } from "@src/lib/powerState";


export function BaseStation({ station, onSelect, selected, editMode }: { station: LighthouseStation, onSelect?: () => void, selected: boolean, editMode?: boolean }) {

    const isAwoke = isAwake(station.power_state);
    const send = useWebsocketCommunication();

    const [identitfyDisabled, setIdentitfyhDisabled] = useState(false);
    const identitify = async () => {

        let result = await IdentitifyBaseStation(station.id);

        if (result != "ok") return alert(result);

        console.log("Identitfy packet sent");
        setIdentitfyhDisabled(true);
        setTimeout(() => {
            setIdentitfyhDisabled(false);
        }, 20000)
    }

    const [powerBusy, setPowerBusy] = useState(false);

    // Explicit commands rather than one toggle: on Windows the power
    // characteristic is write-only, so the app can't tell whether a station is
    // on. A toggle would have to guess, and a wrong guess sends the opposite of
    // what was wanted.
    const setPower = async (mode: "awake" | "sleep") => {
        if (powerBusy) return;

        // A base station takes tens of seconds to spin up and shows nothing for
        // it here, so an impatient second click used to abort the boot.
        setPowerBusy(true);
        setTimeout(() => setPowerBusy(false), 15000);

        await ChangeBaseStationPowerStatus(station.id, mode);
    }

    const { t } = useTranslation();

    const baseStationStatus = useMemo(() => {

        if (station.status == "error") return "error";

        if (station.status != "ready") return "preloaded"

        // Don't claim the station is asleep when we simply never managed to
        // read its power state - show it as unknown instead.
        if (isUnknown(station.power_state)) return "unknown"

        return isAwoke ? "awoke" : "sleep"
    }, [station.power_state, station.status])

    return (<div className={`text-white flex flex-row justify-between poppins-medium bg-[#1F1F1F] rounded-sm p-[16px] items-center hover:bg-[#434343] data-[selected="true"]:bg-[#434343] duration-200 cursor-pointer active:bg-[#1F1F1F]!`} data-selected={selected} onClick={() => {
        if (onSelect && editMode) onSelect();
    }}>
        <div className="flex flex-row gap-[16px] items-center">
            <BaseStationIcon />
            <div className="flex flex-col gap-[2px] text-[14px]">
                <span className="flex flex-row gap-[6px] items-center">
                    <span>{station.name} </span>
                    <StatusCircleIcon class={`data-[status="sleep"]:fill-red-500 data-[status="preloaded"]:fill-blue-500 data-[status="awoke"]:fill-green-500 data-[status="unknown"]:fill-neutral-500 duration-300`} data-status={baseStationStatus} />

                </span>
                <AnimatePresence mode="wait">
                    {station.status == "ready" && <motion.span className="text-[#C6C6C6]" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
                        {t("Channel")} {station.channel}
                    </motion.span>}

                    {station.status != "ready" && <motion.span className="text-[#C6C6C6]" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
                        Connecting...
                    </motion.span>}


                </AnimatePresence>

            </div>
        </div>

        <div className="flex flex-row gap-[8px] [&>*]:flex [&>*]:items-center">
            <AnimatePresence>


                {isAwoke && station.status == "ready" && !editMode ? <motion.div key={"identitfy"} initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
                    <button className={"opacity-75 hover:opacity-100 duration-150 disabled:opacity-25 cursor-pointer p-1 border-[#C6C6C6] border-none rounded-md"} onClick={identitify} disabled={identitfyDisabled}>
                        <Eye color="#C6C6C6" strokeWidth={2} />
                    </button>
                </motion.div>
                    : null}

                {!editMode && <motion.div key={"awoke"} initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="flex flex-row gap-[8px]">
                    {station.status == "ready" && <>
                        <button className="opacity-75 hover:opacity-100 duration-150 disabled:opacity-25 cursor-pointer p-1 border-[#C6C6C6] border-none rounded-md" onClick={() => setPower("awake")} disabled={powerBusy} title={t("Turn on")}>
                            <Power color="#C6C6C6" strokeWidth={2} />
                        </button>
                        <button className="opacity-75 hover:opacity-100 duration-150 disabled:opacity-25 cursor-pointer p-1 border-[#C6C6C6] border-none rounded-md" onClick={() => setPower("sleep")} disabled={powerBusy} title={t("Turn off")}>
                            <PowerOff color="#C6C6C6" strokeWidth={2} />
                        </button>
                    </>}
                </motion.div>}

                {!editMode && <motion.div key={"open"} initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
                    <button key={"Settings"} className="opacity-75 hover:opacity-100 duration-150 disabled:opacity-25 cursor-pointer p-1 border-[#C6C6C6] border-none rounded-md" onClick={() => route(`/devices/${station.id}`)} >
                        <ChevronRightIcon />
                    </button>
                </motion.div>}

            </AnimatePresence>
        </div>
    </div>)
}