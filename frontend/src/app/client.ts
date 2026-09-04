import type * as api from "../api";

export type Client = Pick<typeof api,
  "runtimeMode" | "getHealth" | "getEngines" | "getDefaults" | "listProfiles" |
  "getProfile" | "putProfile" | "deleteProfile" | "testConnection" | "listDatabases" |
  "exportDump" | "importDump" | "listDumps" | "deleteDump" | "downloadDump" |
  "pickOpenPath" | "openBackupsDir"
>;
