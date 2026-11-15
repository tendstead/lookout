export type Server = {
  id: string;
  name: string;
  scan_hours: number;
  stale_hours: number;
  status: string;
  last_success: string | null;
};
export type Availability = {
  server_id: string;
  name: string;
  status: string;
  last_success: string | null;
  files: number;
  seasons: number[];
};
export async function api(url: string, body?: unknown) {
  const r = await fetch(url, {
    method: body === undefined ? 'GET' : 'POST',
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
    cache: 'no-store',
    signal: AbortSignal.timeout(15000),
  });
  if (!r.ok) throw new Error('Request failed');
  return r.json();
}
