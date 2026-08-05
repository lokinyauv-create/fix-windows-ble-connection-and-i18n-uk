import { useRouter } from "preact-router";
import { useState } from "preact/hooks";
import { GroupedBaseStations } from "../assets/icons/GroupedBaseStations";
import { useGroupedLighthouses } from "@src/lib/hooks/useGroupedLighthouses";
import type { LighthouseGroup } from "@src/lib/types";
import { useTranslation } from "react-i18next";
import { AnimatePresence, motion } from "framer-motion";
import { Power, PowerOff, Settings } from "lucide-preact";
import { ChangeBaseStationPowerStatus } from "@src/lib/native/index";
export function BaseStationGroup({ group, id }: { id: string, group: LighthouseGroup }) {

    const baseStations = useGroupedLighthouses(id);
    const [, push] = useRouter();
    const { t } = useTranslation();


    const [powerBusy, setPowerBusy] = useState(false);

    // Explicit commands rather than one toggle - the app can't read a station's
    // power state on Windows, so a toggle would have to guess. See BaseStation.
    const setPower = async (e: MouseEvent, mode: "awake" | "sleep") => {
        e.stopPropagation();

        if (powerBusy) return;

        // Base stations take tens of seconds to spin up. Without a cooldown an
        // impatient second click aborts the boot - same guard as BaseStation.
        setPowerBusy(true);
        setTimeout(() => setPowerBusy(false), 15000);

        for (const bs of baseStations) {
            await ChangeBaseStationPowerStatus(bs.id, mode);
        }
    }

    return (<div className={`text-white flex flex-row justify-between poppins-medium bg-[#1F1F1F] rounded-sm p-[16px] items-center hover:bg-[#434343] data-[selected="true"]:bg-[#434343] duration-200 cursor-pointer active:bg-[#1F1F1F]!`}>
        <div className="flex flex-row gap-[16px] items-center" onClick={() => {
            // onSelect();
            push(`/groups/${id}`);
        }}>
            <GroupedBaseStations />
            <div className="flex flex-col gap-[2px] text-[14px]">
                <span className="flex flex-row gap-[6px] items-center">
                    <span>{group.name}</span>
                    {/* <StatusCircleIcon class={`data-[status="sleep"]:fill-red-500 data-[status="preloaded"]:fill-blue-500 data-[status="awoke"]:fill-green-500 duration-300`}  data-status={baseStationStatus} /> */}

                </span>
                <span className="text-[#C6C6C6]">
                    {baseStations.length > 0 && <span>{t("Channels")}: {baseStations.map(c => c.channel).join(", ")}</span>}
                </span>
            </div>
        </div>

        <div className="flex flex-row gap-[8px] [&>*]:flex [&>*]:items-center">
            <AnimatePresence>

            <motion.div key={"awoke"} className="flex flex-row gap-[8px]">
                <button className="opacity-75 hover:opacity-100 duration-150 disabled:opacity-25 cursor-pointer p-1 border-[#C6C6C6] border-none rounded-md" onClick={(e) => setPower(e, "awake")} disabled={powerBusy} title={t("Turn on")}>
                   <Power color="#C6C6C6" strokeWidth={2}  />
                </button>
                <button className="opacity-75 hover:opacity-100 duration-150 disabled:opacity-25 cursor-pointer p-1 border-[#C6C6C6] border-none rounded-md" onClick={(e) => setPower(e, "sleep")} disabled={powerBusy} title={t("Turn off")}>
                   <PowerOff color="#C6C6C6" strokeWidth={2}  />
                </button>
            </motion.div>

            <button key={"Settings"} className="opacity-75 hover:opacity-100 duration-150 disabled:opacity-25 cursor-pointer p-1 border-[#C6C6C6] border-none rounded-md" onClick={() => push(`/groups/${id}`)} >
                <Settings color="#C6C6C6" strokeWidth={2} />
            </button>
            </AnimatePresence>
        </div>
    </div>)
}