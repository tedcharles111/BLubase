export interface Project {
  id: string;
  name: string;
  ref: string;
  status: 'active' | 'paused' | 'restoring' | 'deploying';
  region: string;
  db_url: string;
  anon_key: string;
  service_key: string;
  created_at: string;
}
export interface DatabaseTable {
  name: string;
  schema: string;
  rows_count: number;
  columns: { name: string; type: string; is_nullable: boolean; is_primary: boolean; default_value: string | null }[];
  rows: Record<string, any>[];
}
export interface StorageFile {
  name: string;
  size: number;
  mime_type: string;
  updated_at: string;
  url: string;
}
export interface EdgeFunction {
  id: string;
  name: string;
  status: 'active' | 'deploying' | 'error';
  deployed_at: string;
  code: string;
  secrets: Record<string, string>;
  logs: string[];
}
export interface AuthProviderConfig {
  id: string;
  name: string;
  enabled: boolean;
  client_id: string;
  client_secret: string;
  redirect_url: string;
}
export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  text: string;
  timestamp: string;
}
