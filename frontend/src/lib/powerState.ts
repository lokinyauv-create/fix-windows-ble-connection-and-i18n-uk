// Power state values reported by a Lighthouse 2.0 base station.
export const POWER_STATE_SLEEP = 0x00;
export const POWER_STATE_STANDBY = 0x02;
export const POWER_STATE_AWAKE = 0x09;
export const POWER_STATE_AWAKE_ALT = 0x0b;
export const POWER_STATE_AWAKE_BOOTING = 0x01;

/**
 * -1 means the base station never reported a power state. This is the normal
 * case on Windows, where the power characteristic doesn't advertise the read
 * property and doesn't support notifications, so there is no way to learn the
 * real state until we ourselves write one.
 */
export const POWER_STATE_UNKNOWN = -1;

export function isAwake(powerState: number): boolean {
    return [POWER_STATE_AWAKE, POWER_STATE_AWAKE_ALT, POWER_STATE_AWAKE_BOOTING].includes(powerState);
}

export function isUnknown(powerState: number): boolean {
    return powerState === POWER_STATE_UNKNOWN;
}
