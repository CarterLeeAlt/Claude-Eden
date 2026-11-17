import { z } from "zod";
import * as Wire from "./contracts.generated.ts";

const strings = z
  .array(z.string())
  .nullable()
  .transform((value) => value ?? []);
export const okSchema = z.object({ ok: z.boolean().optional() });
export const proxyNodeSchema = z.object({
  id: z.string(),
  name: z.string(),
  type: z.string(),
  server: z.string(),
  port: z.number(),
  username: z.string(),
  auth_method: z.string(),
  host_key: strings,
  has_password: z.boolean(),
  has_private_key: z.boolean(),
  has_private_key_passphrase: z.boolean(),
});
export const proxySchema = z.object({
  nodes: z.array(proxyNodeSchema),
  selected: z.string(),
  port: z.number(),
  running: z.boolean(),
  active: z.boolean(),
  state: z.string(),
  state_label: z.string(),
  network_open: z.boolean(),
  core_available: z.boolean(),
  version: z.string(),
  revision: z.string(),
  bootstrap_dns: z.string(),
  udp_blocked: z.boolean(),
  ipv6_blocked: z.boolean(),
});
export type ProxyState = z.infer<typeof proxySchema>;
export type ProxyNode = z.infer<typeof proxyNodeSchema>;
export const proxyConnectionsSchema = z.object({
  uploadTotal: z.number(),
  downloadTotal: z.number(),
  connections: z.array(
    z.object({
      id: z.string(),
      host: z.string(),
      port: z.string(),
      network: z.string(),
      upload: z.number(),
      download: z.number(),
    }),
  ),
});
export type ProxyConnections = z.infer<typeof proxyConnectionsSchema>;
export const statusSchema = z.object({
  projects: strings,
  project_groups: z.array(Wire.WorkbenchProjectGroupSchema),
  ready: z.boolean(),
  vm_running: z.boolean(),
  workspace: z.string(),
  environment_directory: z.string(),
  environment_identity_directory: z.string(),
  local_vm_storage: z.boolean(),
  project_error: z.string(),
  project_errors: z
    .array(
      z.object({
        path: z.string(),
        message: z.string(),
        recovery: z.string().optional(),
      }),
    )
    .nullable()
    .transform((value) => value ?? []),
  storage_recovery: Wire.WorkbenchStorageRecoveryViewSchema,
  keyboard: z.string(),
  auto_timezone: z.boolean(),
  job: Wire.WorkbenchJobSchema,
  location: Wire.WorkbenchLocationSchema.nullable(),
  network_error: z.string(),
  can_restore: z.boolean(),
  safety: Wire.WorkbenchSafetyStateSchema,
  environment_update_required: z.boolean(),
  environment_update_error: z.string(),
  guest_system: z.string().optional(),
  guest_system_error: z.string().optional(),
});
export type Status = z.infer<typeof statusSchema>;
export const terminalSchema = z.object({
  id: z.string(),
  title: z.string(),
  closed: z.boolean(),
  workspace: z.string(),
  kind: z.string(),
  claude_account_id: z.string(),
  claude_account_source: z.string(),
  cli_session_id: z.string(),
  cli_starting: z.boolean(),
  pinned: z.boolean().optional(),
  custom_title: z.string().optional(),
  metadata_set: z.boolean().optional(),
});
export const tabSchema = z.object({
  id: z.string(),
  kind: z
    .string()
    .optional()
    .transform((value) => value ?? "web"),
  title: z.string().optional(),
  url: z.string().optional(),
  workspace: z.string().optional(),
  incognito: z.boolean().optional(),
  loading: z.boolean().optional(),
  document: z.string().optional(),
  favicon: z.string().optional(),
  canGoBack: z.boolean().optional(),
  canGoForward: z.boolean().optional(),
  native_window: z.boolean().optional(),
  login: z.boolean().optional(),
});
export const downloadSchema = z.object({
  id: z.string(),
  name: z.string().optional(),
  state: z.string(),
  url: z.string().optional(),
  target: z.string().optional(),
  path: z.string().optional(),
  receivedBytes: z.number().optional(),
  totalBytes: z.number().optional(),
  startedAt: z.string().optional(),
  openError: z.string().optional(),
  error: z.string().optional(),
  retryable: z.boolean().optional(),
  mime: z.string().optional(),
});
export const uploadSchema = z.object({
  id: z.string(),
  kind: z.string().optional(),
  multiple: z.boolean().optional(),
  directory: z.boolean().optional(),
  target: z.string().optional(),
  accept: strings.optional(),
});
export type Download = z.infer<typeof downloadSchema>;
export type Upload = z.infer<typeof uploadSchema>;
export const browserSchema = z.object({
  tabs: z.array(tabSchema),
  active: z.string().optional(),
  split: z
    .object({
      left: z.string(),
      right: z.string(),
      fraction: z.number().optional(),
      history: z
        .record(
          z.string(),
          z.object({ back: z.boolean(), forward: z.boolean() }),
        )
        .optional(),
    })
    .nullable()
    .optional(),
  downloads: z.array(downloadSchema).optional(),
  uploads: z.array(uploadSchema).optional(),
  focus: z.boolean().optional(),
  desktopFocus: z.boolean().optional(),
  back: z.boolean().optional(),
  forward: z.boolean().optional(),
  capabilities: z
    .object({ systemClipboard: z.boolean().optional() })
    .optional(),
});
export type BrowserState = z.infer<typeof browserSchema>;
const storageLocationSchema = z.object({
  directory: z.string(),
  suggested_directory: z.string().optional(),
  has_environment: z.boolean(),
  backup: z.string().optional(),
  recovery: Wire.WorkbenchStorageRecoveryViewSchema.optional(),
});
export const storageSchema = z.union([
  storageLocationSchema,
  storageLocationSchema.partial().extend({
    recovery: Wire.WorkbenchStorageRecoveryViewSchema.extend({
      required: z.literal(true),
    }),
  }),
]);
export type StorageLocation = z.infer<typeof storageSchema>;
export const vncSchema = z.object({
  password: z.string(),
  display_scale: z.number().optional(),
  dynamic_scale: z.boolean().optional(),
  pixel_wheel: z.boolean().optional(),
});
export const copySchema = z.object({
  id: z.string().optional(),
  text: z.string().optional(),
  clipboard: z
    .object({ id: z.string(), mime: z.string(), size: z.number() })
    .optional(),
  context: z.string().optional(),
  document: z.string().optional(),
  error: z.string().optional(),
});
export const clipboardEventSchema = copySchema.required({
  id: true,
  context: true,
  text: true,
});
export const terminalEventSchema = z.discriminatedUnion("type", [
  z.object({ type: z.literal("cli-startup"), pending: z.boolean() }),
  z.object({ type: z.literal("history") }),
]);
export const audioFormatSchema = z.object({
  format: z.string(),
  sample_rate: z.number(),
  channels: z.number(),
});
export const inputSchema = z.object({
  available: z.boolean(),
  context: z.string().optional(),
  rect: z
    .tuple([z.number(), z.number(), z.number(), z.number()])
    .nullable()
    .optional(),
  session: z.string().optional(),
  x: z.number().optional(),
  y: z.number().optional(),
  width: z.number().optional(),
  height: z.number().optional(),
  pid: z.number().optional(),
  window: z.string().optional(),
  surface: z.string().optional(),
});
export type InputState = z.infer<typeof inputSchema>;
export const resourcesSchema = z.object({
  running: z.boolean(),
  automatic: z.boolean(),
  memory_gib: z.number(),
  cpus: z.number(),
  min_memory_gib: z.number().optional(),
  min_cpus: z.number().optional(),
  max_memory_gib: z.number(),
  max_cpus: z.number(),
  resource_error: z.string(),
  recommended_memory_bytes: z.number(),
  recommended_cpus: z.number(),
  configured_memory_bytes: z.number(),
  configured_cpus: z.number(),
  restart_required: z.boolean(),
  observation: Wire.EdenResourceStatusSchema.nullable(),
});
export type Resources = z.infer<typeof resourcesSchema>;
export const versionsSchema = z.object({
  vm_running: z.boolean(),
  busy: z.boolean(),
  components: Wire.WorkbenchComponentVersionsSchema,
  controller: z.object({ version: z.string() }),
  tools: z.record(z.string(), Wire.WorkbenchToolVersionSchema),
  tools_update_allowed: z.boolean(),
  error: z.string(),
});
export type Versions = z.infer<typeof versionsSchema>;
export const sessionsSchema = z.object({
  sessions: z.array(Wire.WorkbenchCliSessionSummarySchema),
  notice: z.string().optional(),
});
export const sessionAccountSchema = z.object({
  supported: z.boolean(),
  source_id: z.string(),
  switch_policy: z.string(),
  session: Wire.WorkbenchCliSessionSchema,
  candidates: z.array(z.object({ id: z.string(), label: z.string() })),
});
export type SessionAccountView = z.infer<typeof sessionAccountSchema>;
export const cookieSchema = z.object({
  key: z.string(),
  name: z.string(),
  domain: z.string(),
  path: z.string(),
  size: z.number(),
  session: z.boolean(),
  expires: z.number(),
  secure: z.boolean(),
  httpOnly: z.boolean(),
  sameSite: z.string().optional(),
  partitionKeyOpaque: z.boolean().optional(),
  partitionKey: z.object({ topLevelSite: z.string() }).optional(),
});
export const cookiesSchema = z.object({
  cookies: z.array(cookieSchema),
  hostname: z.string().optional(),
  page: z.number(),
  total: z.number(),
});
export const cookieValueSchema = z.object({ value: z.string() });
export const vaultBrowserSchema = z.union([
  Wire.WorkbenchVaultBrowserStateSchema,
  z
    .object({
      available: z.literal(false),
      message: z.string(),
      candidates: z.array(Wire.WorkbenchVaultCandidateSchema),
      entries: z.array(Wire.WorkbenchVaultEntrySchema),
    })
    .transform((value) => ({ ...value, origin: "", document: "" })),
]);
export type Cookies = z.infer<typeof cookiesSchema>;
export const guestProfileSchema = z.object({
  mode: z.string(),
  hostname: z.string(),
  username: z.string(),
  work_home: z.string(),
  browser_username: z.string(),
  browser_home: z.string(),
  extension_name: z.string(),
  display_width: z.number(),
  display_height: z.number(),
  browser_width: z.number(),
  browser_height: z.number(),
  audio_description: z.string(),
  audio_model: z.string(),
  audio_name: z.string(),
  graphics_backend: z.string(),
  webgl: z.string(),
  canvas: z.string(),
});
export type GuestProfile = z.infer<typeof guestProfileSchema>;
export const guestProfileRecordSchema = z.object({
  editable: z.boolean(),
  defaults: guestProfileSchema,
  profile: guestProfileSchema.nullable().optional(),
});
export type GuestProfileRecord = z.infer<typeof guestProfileRecordSchema>;
export const browserPreferencesSchema = z.object({
  browser_home_url: z.string(),
});
export type BrowserPreferences = z.infer<typeof browserPreferencesSchema>;
export const extensionSchema = z.object({
  id: z.string(),
  name: z.string(),
  version: z.string(),
  description: z.string().optional(),
  enabled: z.boolean().optional(),
  mayDisable: z.boolean().optional(),
  managed: z.boolean().optional(),
  source: z.string().optional(),
  error: z.string().optional(),
  optionsUrl: z.string().optional(),
  permissions: strings.optional(),
  hostPermissions: strings.optional(),
});
export type BrowserExtension = z.infer<typeof extensionSchema>;
export const extensionPreviewSchema = extensionSchema
  .omit({ id: true })
  .extend({ session: z.string() });
export type ExtensionPreview = z.infer<typeof extensionPreviewSchema>;
export const cacheInventorySchema = Wire.WorkbenchCacheInventorySchema.extend({
  protected: z.array(
    z.object({
      title: z.string(),
      present: z.number(),
      unknown: z.number().optional(),
    }),
  ),
});
export const cacheResultSchema = z.object({
  status: z.string(),
  outcomes: z.array(z.object({ id: z.string(), reason: z.string() })),
});
export type CacheResult = z.infer<typeof cacheResultSchema>;
export const tailnetPeerSchema = z.object({
  id: z.string().optional(),
  hostname: z.string().optional(),
  name: z.string().optional(),
  ip: z.string(),
  os: z.string().optional(),
  online: z.boolean(),
});
export const tailscaleSchema = z.object({
  enabled: z.boolean(),
  installed: z.boolean(),
  backend: z.string(),
  auth_url: z.string().optional(),
  return_id: z.string().optional(),
  error: z.string().optional(),
  ips: strings.optional(),
  peers: z.array(tailnetPeerSchema).optional(),
  tailnet: z.string().optional(),
  account: z.string().optional(),
  health: strings.optional(),
  os: z.string().optional(),
  hostname: z.string().optional(),
  name: z.string().optional(),
  forwards: z
    .array(
      z.object({ host: z.string(), port: z.number(), address: z.string() }),
    )
    .nullable()
    .transform((value) => value ?? [])
    .optional(),
  user: z.string().optional(),
  keys: strings.optional(),
  proxy_url: z.string().optional(),
});
export type Tailnet = z.infer<typeof tailscaleSchema>;

export const responseSchemas = {
  "/session": okSchema,
  "/account/suspend": okSchema,
  "/client/cancel": okSchema,
  "/client/commit": okSchema,
  "/status": statusSchema,
  "/cli/sessions": sessionsSchema,
  "/cli/sessions?archived=true": sessionsSchema,
  "/settings": Wire.WorkbenchSettingsSchema,
  "/proxy": proxySchema,
  "/proxy/connections": proxyConnectionsSchema,
  "/client/about": Wire.VersionBuildSchema,
  "/appearance": Wire.WorkbenchAppearanceSchema,
  "/appearance/codex": Wire.WorkbenchAppearanceSchema,
  "/account": Wire.WorkbenchAccountViewSchema,
  "/client/context": Wire.WorkbenchClientContextSchema,
  "/client/candidate": Wire.WorkbenchDesktopCandidateSchema,
  "/favorites": Wire.WorkbenchFavoritesSchema,
  "/browser/tabs": browserSchema,
  "/browser/preferences": browserPreferencesSchema,
  "/browser/info": z.object({ product: z.string() }),
  "/browser/extensions": z.object({
    items: z.array(extensionSchema).optional(),
    native: z.boolean().optional(),
    id: z.string().optional(),
    url: z.string().optional(),
    kind: z.string().optional(),
    title: z.string().optional(),
  }),
  "/browser/split": browserSchema.partial(),
  "/browser/new": tabSchema,
  "/browser/editor": tabSchema,
  "/browser/settings": tabSchema,
  "/browser/internal-page": tabSchema,
  "/browser/activate": okSchema,
  "/browser/close": okSchema,
  "/browser/navigate": okSchema,
  "/browser/reload": okSchema,
  "/browser/back": okSchema,
  "/browser/forward": okSchema,
  "/browser/editor-restart": okSchema,
  "/environment/storage": storageSchema,
  "/environment/resources": resourcesSchema,
  "/environment/versions": versionsSchema,
  "/environment/update": Wire.WorkbenchJobSchema,
  "/environment/tools": Wire.WorkbenchJobSchema,
  "/environment/guest-profile": guestProfileRecordSchema,
  "/environment/guest-profile/preview": guestProfileSchema,
  "/environment/initialization": Wire.WorkbenchInitializationViewSchema,
  "/environment/device-check": Wire.WorkbenchDeviceCheckReportSchema,
  "/environment/cache/inventory": cacheInventorySchema,
  "/claude-desktop": Wire.WorkbenchClaudeDesktopStateSchema,
  "/claude-accounts": Wire.WorkbenchClaudePanelSchema,
  "/claude-accounts?panel=visible": Wire.WorkbenchClaudePanelSchema,
  "/client/browser-extensions": z.object({
    profiles: z.array(Wire.WorkbenchLocalBrowserProfileSchema),
  }),
  "/vault": Wire.WorkbenchVaultDataSchema,
  "/vault/browser": vaultBrowserSchema,
  "/audio": audioFormatSchema.extend({
    ready: z.boolean(),
  }),
  "/vnc-credentials": vncSchema,
  "/claude-desktop/vnc-credentials": vncSchema,
  "/browser/wheel": z.object({ handled: z.boolean() }),
  "/browser/resize": z.object({ scale: z.number().optional() }),
  "/claude-desktop/resize": z.object({ scale: z.number().optional() }),
  "/browser/copy": copySchema,
  "/browser/cut": copySchema,
  "/input": inputSchema,
  "/proxy/delay": z.object({ delay: z.number() }),
  "/proxy/logs": z.object({ lines: z.array(z.string()) }),
  "/diagnostics": z.object({ bytes: z.number(), files: z.number() }),
  "/client/trusted-login": okSchema,
  "/projects/remove": z.object({ workspace: z.string() }),
  "/projects/save": z.object({ workspace: z.string() }),
  "/projects/open": z.object({ workspace: z.string() }),
  "/browser/cookies": cookiesSchema,
  "/browser/upload-finish": okSchema,
  "/browser/upload-cancel": okSchema,
  "/browser/download-retry": okSchema,
  "/browser/copy-context": okSchema,
  "/browser/insert-text": z.object({ available: z.boolean().optional() }),
  "/input/files/directory": okSchema,
  "/input/files/download-retry": okSchema,
  "/input/files/desktop/open": okSchema,
  "/browser/clipboard": z.object({
    text: z.string().optional(),
    png: z.string().optional(),
  }),
  "/input/files/desktop/requests": z.object({
    uploads: z.array(uploadSchema),
    downloads: z.array(downloadSchema),
    opens: z.array(z.object({ id: z.string() })).optional(),
  }),
  "/browser/files/saved": z.array(Wire.WorkbenchSavedDownloadSchema),
  "/agent/native/window": z.object({ window_proof: z.string().min(1) }),
  "/input/files/start": z.object({ id: z.string() }),
  "/input/files/finish": okSchema,
  "/input/files/cancel": okSchema,
  "/input/paste/cancel": okSchema,
  "/input/paste/start": z.object({
    available: z.boolean(),
    transfer: z.string().optional(),
    context: z.string().optional(),
  }),
  "/input/image": z.object({ path: z.string().optional() }),
  "/input/text": z.object({ available: z.boolean().optional() }),
  "/settings/probe": Wire.WorkbenchLocationSchema,
  "/start": Wire.WorkbenchJobSchema,
  "/stop": Wire.WorkbenchJobSchema,
  "/tailscale/install": Wire.WorkbenchJobSchema,
  "/safety/check": Wire.WorkbenchJobSchema,
  "/safety/close": Wire.WorkbenchJobSchema,
  "/environment/storage/recover": Wire.WorkbenchJobSchema,
  "/restore": Wire.WorkbenchJobSchema,
  "/projects/refresh": okSchema,
  "/tailscale": tailscaleSchema,
} as const;
export const mutationSchemas = {
  "/settings": Wire.WorkbenchJobSchema,
  "/environment/storage": Wire.WorkbenchJobSchema,
  "/environment/resources": z.union([Wire.WorkbenchJobSchema, okSchema]),
  "/client/about": Wire.AppupdateAboutCheckSchema,
} as const;
export type MutationPath = keyof typeof mutationSchemas;
export type MutationResponse<P extends MutationPath> = z.infer<
  (typeof mutationSchemas)[P]
>;
export type KnownPath = keyof typeof responseSchemas;
export type ApiResponse<P extends KnownPath> = z.infer<
  (typeof responseSchemas)[P]
>;

export function responseSchemaFor(
  path: string,
  method: string,
  body?: unknown,
): z.ZodType | undefined {
  if (
    path === "/vault/browser" &&
    method === "POST" &&
    body !== null &&
    typeof body === "object" &&
    "action" in body &&
    body.action !== "status"
  )
    return okSchema;
  if (
    path === "/browser/cookies" &&
    method === "POST" &&
    body !== null &&
    typeof body === "object" &&
    "mode" in body &&
    body.mode === "details"
  )
    return cookieValueSchema;
  if (method === "POST" && Object.hasOwn(mutationSchemas, path))
    return mutationSchemas[path as MutationPath];
  if (Object.hasOwn(responseSchemas, path))
    return responseSchemas[path as KnownPath];
  if (path === "/terminals")
    return method === "GET" ? z.array(terminalSchema) : terminalSchema;
  if (/^\/terminals\/[^/]+\/metadata$/.test(path)) return terminalSchema;
  if (/^\/terminals\/[^/]+\/login-link$/.test(path)) return tabSchema;
  if (/^\/cli\/sessions\/[^/]+\/open$/.test(path))
    return Wire.WorkbenchCliSessionSchema;
  if (/^\/cli\/sessions\/[^/]+\/account$/.test(path))
    return sessionAccountSchema;
  if (
    /^\/proxy\/(add|edit|delete|select|stop|restart)$/.test(path) ||
    /^\/claude-desktop\/(start|stop|install|update)$/.test(path)
  )
    return Wire.WorkbenchJobSchema;
  if (
    /^\/tailscale\/(connect|login|disconnect|logout|open-service|add-key|remove-key)$/.test(
      path,
    )
  )
    return tailscaleSchema;
  if (/^\/environment\/cache\/(clean|restore)$/.test(path))
    return cacheResultSchema;
  if (/^\/account\/[^/]+$/.test(path))
    return Wire.WorkbenchAccountViewSchema.extend({
      recovery_key: z.string().optional(),
    });
  if (path === "/agent/native?consumer=window")
    return z
      .object({
        id: z.string(),
        connection_id: z.string(),
        request: z.record(z.string(), z.unknown()),
      })
      .nullable();
  if (
    /^\/cli\/sessions\/[^/]+\/(metadata|archive|purge|restore)$/.test(path) ||
    /^\/agent\/native\/[^/]+\/complete$/.test(path) ||
    /^\/terminals\/[^/]+$/.test(path)
  )
    return okSchema;
  return undefined;
}

export function parseResponse<T>(
  schema: z.ZodType<T>,
  value: unknown,
  path: string,
): T {
  const parsed = schema.safeParse(value);
  if (!parsed.success)
    throw new Error(
      `后台返回的数据结构无效（${path}）。请更新 Claude Eden；仍失败时导出诊断包。`,
      { cause: parsed.error },
    );
  return parsed.data;
}
