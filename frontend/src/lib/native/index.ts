import { useContext } from "preact/hooks";
import { useConfig } from "../hooks/useConfig";
import { usePlatform } from "../hooks/usePlatform";
import { WebsocketContext, type WebsocketContextType } from "../context/websocket.context";

import * as native from "../../../wailsjs/go/main/App"
type status = "ok" | string

export const ChangeBaseStationPowerStatus = async (id: string, mode: "sleep" | "awake") => {
    await native.ChangeBaseStationPowerStatus(id, mode);
}

export const RenameGroup = async (old: string, newName: string) => {
    return native.RenameGroup(old, newName);
}

export const UpdateGroupManagedFlags = async (group: string, flags: number) => {
    return native.UpdateGroupManagedFlags(group, flags);
}

export const IdentitifyBaseStation = async (id: string): Promise<status> => {
    return native.IdentitifyBaseStation(id);
}

export const ForceUpdate = async (): Promise<void> => {
    return native.ForceUpdate();
}

export const GetConfiguration = async (): Promise<any> => {
    return {};
}


export const IsSteamVRConnectivityAvailable = async (): Promise<boolean> => {
    const config = useConfig();
    return config.is_steamvr_installed;
}

export const IsUpdatingSupported = async (): Promise<boolean> => {
    const platform = usePlatform();
    return platform.system == "windows";
}

export const UpdateConfigValue = async (key: string, value: any): Promise<void> => {
    return native.UpdateConfigValue(key, value);
}

export const AddBaseStationToGroup = async (stationId: string, groupId: string): Promise<status> => {
    return native.AddBaseStationToGroup(stationId, groupId);
}

export const CreateGroup = async (name: string, baseStations?: Array<string>): Promise<string> => {
    return native.CreateGroup(name, baseStations ?? []);
}

export const InitBluetooth = async (): Promise<status> => {
    return "ok";
}

export const ChangeBaseStationChannel = async (id: string, channel: number): Promise<status> => {
    return native.ChangeBaseStationChannel(id, channel);
}

export const ForgetBaseStation = async (id: string): Promise<void> => {
    return native.ForgetBaseStation(id);
}

export const UpdateBaseStationParam = async (id: string, param: string, value: any): Promise<void> => {
    return native.UpdateBaseStationParam(id, param, value);
}

export const RemoveGroup = async (id: string): Promise<void> => {
    return native.RemoveGroup(id);
}
export interface PlayAreaZone {
    id: string,
    name: string,
    play_area: number[] | null,
    created_at: string,
    auto_apply: boolean
}

export interface RoomStatus {
    supported: boolean,
    steamvr_running: boolean,
    helper_found: boolean,
    room_setup_found: boolean,
    watcher_running: boolean
}

export const GetRoomStatus = async (): Promise<RoomStatus> => {
    return native.GetRoomStatus() as Promise<RoomStatus>;
}

export const ListZones = async (): Promise<PlayAreaZone[]> => {
    return (await native.ListZones() ?? []) as PlayAreaZone[];
}

export const GetZoneLog = async (): Promise<string[]> => {
    return (await native.GetZoneLog()) ?? [];
}

export const CaptureZone = async (name: string): Promise<status> => {
    return native.CaptureZone(name);
}

export const ApplyZone = async (id: string): Promise<status> => {
    return native.ApplyZone(id);
}

export const RenameZone = async (id: string, name: string): Promise<status> => {
    return native.RenameZone(id, name);
}

export const DeleteZone = async (id: string): Promise<status> => {
    return native.DeleteZone(id);
}

export const SetAutoZone = async (id: string): Promise<status> => {
    return native.SetAutoZone(id);
}

export const LaunchRoomSetup = async (): Promise<status> => {
    return native.LaunchRoomSetup();
}
