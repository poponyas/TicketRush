// The existing API returns bearer + refresh tokens. Keep access in memory and
// refresh in sessionStorage (one browser tab); no passwords or tokens in URLs.
let access = '';
let refreshing;
let epoch = 0;
const key = 'ticketrush.refresh';
const storage = {
  get() { try { return sessionStorage.getItem(key); } catch { return null; } },
  set(value) { try { value ? sessionStorage.setItem(key, value) : sessionStorage.removeItem(key); } catch { /* Current session still works in memory. */ } },
};
export class APIError extends Error {
  constructor(status, data = {}, retryAfter) {
    super(data.message || 'Не удалось выполнить запрос');
    this.status = status; this.code = data.code; this.requestID = data.request_id; this.retryAfter = retryAfter;
  }
}
async function send(path, options) {
  let response;
  try { response = await fetch('/api/v1' + path, { ...options, signal: AbortSignal.timeout(20000), cache: 'no-store', credentials: 'same-origin' }); }
  catch { throw new APIError(0, { message: 'Сервер не отвечает. Проверьте соединение и попробуйте ещё раз.' }); }
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new APIError(response.status, data, response.headers.get('Retry-After'));
  }
  if (response.status === 204) return null;
  return options.blob ? response.blob() : response.json();
}
export function clearSession() { epoch++; access = ''; storage.set(null); if (typeof window !== 'undefined') window.dispatchEvent(new Event('ticketrush:signout')); }
export function setSession(result) { access = result.access_token; storage.set(result.refresh_token); }
export function hasSession() { return Boolean(access || storage.get()); }
async function refresh() {
  if (refreshing) return refreshing;
  const token = storage.get();
  if (!token) throw new APIError(401);
  const started = epoch;
  refreshing = send('/auth/refresh', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ refresh_token: token }) })
    .then(result => { if (started !== epoch) throw new APIError(401); setSession(result); })
    .catch(error => { if (error.status === 401 && started === epoch) clearSession(); throw error; })
    .finally(() => { refreshing = null; });
  return refreshing;
}
export async function api(path, { method = 'GET', data, auth = false, org, blob = false } = {}) {
  if (auth && !access) await refresh();
  const headers = {};
  if (access && auth) headers.Authorization = 'Bearer ' + access;
  if (org) headers['X-Organizer-ID'] = org;
  let body;
  if (data instanceof File) { body = data; headers['Content-Type'] = data.type; }
  else if (data !== undefined) { body = JSON.stringify(data); headers['Content-Type'] = 'application/json'; }
  const options = { method, body, headers, blob };
  try { return await send(path, options); }
  catch (error) {
    if (auth && error.status === 401 && storage.get()) {
      await refresh();
      options.headers.Authorization = 'Bearer ' + access;
      return send(path, options);
    }
    if (auth && error.status === 401) clearSession();
    throw error;
  }
}
export function message(error, fallback) {
  if (error.status === 0) return error.message;
  if (error.status === 429) return `Слишком много запросов. Повторите через ${error.retryAfter || 60} сек.`;
  return fallback || ({
    400: 'Проверьте введённые данные.', 401: 'Войдите в аккаунт, чтобы продолжить.',
    403: 'У этого аккаунта нет доступа к действию.', 404: 'Не удалось найти запрошенные данные.',
    409: 'Данные изменились или действие сейчас недоступно. Обновите страницу.',
    413: 'Файл слишком большой. Максимальный размер — 5 МБ.',
    415: 'Этот формат не поддерживается.', 503: 'Сервис временно недоступен. Попробуйте позже.',
  })[error.status] || 'Не удалось выполнить действие. Попробуйте ещё раз.';
}
