// Starting always requires a node applied by the Eden proxy manager.
export function needsProxySetup(
  _settings: unknown,
  _status: unknown,
  proxy: { active?: boolean; selected?: string } | null,
) {
  return !(proxy?.active && proxy?.selected);
}
