import { useEffect, useRef, useState } from 'preact/hooks'
import { motion } from 'framer-motion'
import { useTranslation } from 'react-i18next';
import { Check, Pencil, Play, ScanLine, Trash2, X } from 'lucide-preact';
import { Checkbox } from '../components/Checkbox';
import { smoothResize } from '../utils/windows';
import { useContainerTitlebar } from '@src/lib/stores/titlebar.store.ts';
import {
    ApplyZone, CaptureZone, DeleteZone, GetRoomStatus, GetZoneLog, LaunchRoomSetup, ListZones,
    RenameZone, SetAutoZone, type PlayAreaZone, type RoomStatus
} from '@src/lib/native/index.ts';

const formatArea = (pa: number[] | null) => pa && pa.length >= 2 ? `${pa[0].toFixed(1)} × ${pa[1].toFixed(1)} м` : "?";
const formatDate = (iso: string) => iso ? iso.replace("T", " ").slice(0, 16) : "";

const iconButton = "opacity-75 hover:opacity-100 duration-150 disabled:opacity-25 disabled:cursor-not-allowed cursor-pointer p-1";
const primaryButton = "text-[12px] py-[6px] px-[16px] bg-[#1D81FF] rounded-[6px] duration-100 hover:bg-[#66AAFF] cursor-pointer disabled:bg-[#2A63AB] disabled:opacity-60 disabled:cursor-not-allowed";
const textInput = "h-[26px] bg-[#121212] text-[#C6C6C6] rounded-md text-[12px] px-[12px] focus:outline-none poppins-medium";

export function PlayAreaView() {
    const { t, i18n } = useTranslation();
    const { setItems } = useContainerTitlebar();
    const [status, setStatus] = useState<RoomStatus | null>(null);
    const [zones, setZones] = useState<PlayAreaZone[]>([]);
    const [log, setLog] = useState<string[]>([]);
    const [newName, setNewName] = useState("");
    const [editing, setEditing] = useState<{ id: string, name: string } | null>(null);
    const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
    const [busy, setBusy] = useState(false);
    const [message, setMessage] = useState<{ text: string, error: boolean } | null>(null);
    const logRef = useRef<HTMLDivElement>(null);

    const refresh = async () => {
        const [s, z, l] = await Promise.all([GetRoomStatus(), ListZones(), GetZoneLog()]);
        setStatus(s);
        setZones(z.sort((a, b) => a.created_at.localeCompare(b.created_at)));
        setLog(l);
    };

    useEffect(() => {
        smoothResize(700, 560);
        refresh();
        const timer = setInterval(refresh, 2000);
        return () => clearInterval(timer);
    }, []);

    useEffect(() => {
        setItems([{ text: "SteamVR LM", link: "/" }, { text: t("Room"), link: "/room" }]);
    }, [i18n.language]);

    useEffect(() => {
        if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight;
    }, [log.length]);

    // Runs a backend action; its "error: ..." strings end up in the message line.
    const run = async (action: () => Promise<string>, success?: string) => {
        setBusy(true);
        setMessage(null);
        try {
            const result = await action();
            if (result && result !== "ok") {
                setMessage({ text: result.replace(/^error:\s*/, ""), error: true });
            } else if (success) {
                setMessage({ text: success, error: false });
            }
        } finally {
            setBusy(false);
            await refresh();
        }
    };

    const steamvr = status?.steamvr_running ?? false;

    return (<div className="poppins-semibold text-white py-[12px] px-[24px] select-none">
        <div className='flex flex-col gap-[8px] w-full pt-[8px]'>

            <div className='flex flex-row justify-between items-center w-full bg-[#1F1F1F] p-[14px] rounded-[6px]'>
                <div className='flex flex-col'>
                    <span className='text-[14px] poppins-regular'>{t("Room Setup")}</span>
                    <span className='text-[12px] opacity-80 poppins-regular'>
                        {!status?.room_setup_found ? t("Room Setup launcher not found")
                            : steamvr ? t("Calibrate the floor and draw the play area (fixed for Linux/AMD)")
                                : t("Start SteamVR first")}
                    </span>
                </div>
                <button className={`${primaryButton} flex flex-row items-center gap-[6px]`} disabled={!steamvr || !status?.room_setup_found || busy}
                    onClick={() => run(LaunchRoomSetup, t("Room Setup started"))}>
                    <ScanLine size={14} /> {t("Launch")}
                </button>
            </div>

            <div className='flex flex-row justify-between items-center w-full bg-[#1F1F1F] p-[14px] rounded-[6px] gap-[12px]'>
                <div className='flex flex-col'>
                    <span className='text-[14px] poppins-regular'>{t("Save current play area")}</span>
                    <span className='text-[12px] opacity-80 poppins-regular'>{steamvr ? t("Takes the zone SteamVR is using right now") : t("Start SteamVR first")}</span>
                </div>
                <div className='flex flex-row gap-[8px] items-center'>
                    <input className={`${textInput} w-[170px]`} maxLength={40} placeholder={t("Zone name")} value={newName}
                        onInput={(e: any) => setNewName(e.target.value)}
                        onKeyDown={(e: any) => { if (e.key === "Enter" && newName.trim() && steamvr) run(() => CaptureZone(newName), t("Zone saved")).then(() => setNewName("")) }} />
                    <button className={primaryButton} disabled={!steamvr || !newName.trim() || busy}
                        onClick={() => run(() => CaptureZone(newName), t("Zone saved")).then(() => setNewName(""))}>
                        {t("Save")}
                    </button>
                </div>
            </div>

            <div className='flex flex-col w-full bg-[#1F1F1F] rounded-[6px] py-[6px] max-h-[170px] overflow-y-auto'>
                {zones.length === 0 && <span className='text-[12px] opacity-60 poppins-regular px-[14px] py-[8px]'>{t("No saved zones yet")}</span>}
                {zones.map(zone => (
                    <div key={zone.id} className='flex flex-row justify-between items-center px-[14px] py-[6px] hover:bg-[#262626] duration-100'>
                        <div className='flex flex-col min-w-0'>
                            {editing?.id === zone.id ?
                                <div className='flex flex-row items-center gap-[4px]'>
                                    <input className={`${textInput} w-[180px]`} maxLength={40} value={editing.name} autoFocus
                                        onInput={(e: any) => setEditing({ id: zone.id, name: e.target.value })}
                                        onKeyDown={(e: any) => {
                                            if (e.key === "Enter") run(() => RenameZone(zone.id, editing.name)).then(() => setEditing(null));
                                            if (e.key === "Escape") setEditing(null);
                                        }} />
                                    <button className={iconButton} onClick={() => run(() => RenameZone(zone.id, editing.name)).then(() => setEditing(null))}><Check size={16} color="#C6C6C6" /></button>
                                    <button className={iconButton} onClick={() => setEditing(null)}><X size={16} color="#C6C6C6" /></button>
                                </div>
                                : <span className='text-[14px] poppins-regular truncate'>{zone.name}</span>}
                            <span className='text-[11px] opacity-60 poppins-regular'>{formatArea(zone.play_area)} · {formatDate(zone.created_at)}</span>
                        </div>
                        <div className='flex flex-row items-center gap-[10px] shrink-0'>
                            <div className='flex flex-row items-center gap-[6px]' title={t("Apply automatically when SteamVR starts and after base station recalibration")}>
                                <span className='text-[11px] opacity-70 poppins-regular'>{t("Auto")}</span>
                                <Checkbox disabled={busy} value={zone.auto_apply} SetValue={(v: boolean) => run(() => SetAutoZone(v ? zone.id : ""))} />
                            </div>
                            <button className={iconButton} disabled={!steamvr || busy} title={t("Apply now")} onClick={() => run(() => ApplyZone(zone.id), t("Zone applied"))}>
                                <Play size={18} color="#C6C6C6" />
                            </button>
                            <button className={iconButton} disabled={busy} title={t("Rename")} onClick={() => setEditing({ id: zone.id, name: zone.name })}>
                                <Pencil size={16} color="#C6C6C6" />
                            </button>
                            <button className={iconButton} disabled={busy} title={confirmDelete === zone.id ? t("Click again to delete") : t("Delete")}
                                onClick={() => {
                                    if (confirmDelete !== zone.id) {
                                        setConfirmDelete(zone.id);
                                        setTimeout(() => setConfirmDelete(c => c === zone.id ? null : c), 3000);
                                        return;
                                    }
                                    setConfirmDelete(null);
                                    run(() => DeleteZone(zone.id));
                                }}>
                                <Trash2 size={16} color={confirmDelete === zone.id ? "#FF7373" : "#C6C6C6"} />
                            </button>
                        </div>
                    </div>
                ))}
            </div>

            {message && <motion.span initial={{ opacity: 0 }} animate={{ opacity: 1 }} className={`text-[12px] poppins-regular ${message.error ? "text-[#FF7373]" : "text-[#7CDB8A]"}`}>{message.text}</motion.span>}

            <div ref={logRef} className='w-full bg-[#121212] rounded-[6px] p-[8px] h-[110px] overflow-y-auto text-[11px] text-[#888888] font-mono select-text'>
                {log.length === 0 ? t("Waiting for SteamVR...") : log.map((line, i) => <div key={i}>{line}</div>)}
            </div>
        </div>
    </div>);
}
