import { api, clearSession, hasSession } from './api.js';
import { e, icon, errorView, loading, toast } from './ui.js';
import { catalog, eventDetail } from './views/catalog.js';
import { authPage, accountPage, bookingPage } from './views/account.js';
import { workspace, manageEvent, venuesPage, checkInPage } from './views/workspace.js';

const main = document.querySelector('#content');
document.querySelector('.skip-link').addEventListener('click', event => { event.preventDefault(); main.focus(); });
const ctx = { profile: null, cleanup: [], reload: route, async loadProfile() { ctx.profile = await api('/me', { auth: true }); updateHeader(); } };
let generation = 0;
window.addEventListener('ticketrush:signout', () => { ctx.profile = null; updateHeader(); });
function updateHeader() {
  const target = document.querySelector('#user-menu');
  target.innerHTML = ctx.profile ? `<a class="profile-link" href="#/account"><span class="avatar">${e(ctx.profile.email[0].toUpperCase())}</span><span class="profile-label">Личный кабинет</span></a><button id="logout" class="text-button">Выйти</button>` : '<a class="button small" href="#/login" aria-label="Войти">Войти ↗</a>';
  target.querySelector('#logout')?.addEventListener('click', async () => {
    const button = target.querySelector('#logout'); button.disabled = true;
    try { await api('/auth/logout', { method: 'POST', auth: true }); clearSession(); ctx.profile = null; updateHeader(); toast('Вы вышли из аккаунта'); location.hash = '/'; await route(); }
    catch { button.disabled = false; toast('Не удалось завершить сессию на сервере. Повторите выход.', true); }
  });
}
async function route() {
  const current = ++generation;
  for (const cleanup of ctx.cleanup.splice(0)) cleanup();
  document.querySelectorAll('dialog').forEach(d => d.close());
  const raw = location.hash.slice(1) || '/', split = raw.indexOf('?');
  const path = split < 0 ? raw : raw.slice(0, split), query = new URLSearchParams(split < 0 ? '' : raw.slice(split + 1));
  const parts = path.split('/').filter(Boolean);
  for (const link of document.querySelectorAll('[data-nav]')) {
    const selected = link.dataset.nav === (parts[0] === 'workspace' ? 'workspace' : ['account', 'bookings'].includes(parts[0]) ? 'account' : 'catalog');
    link.classList.toggle('active', selected); selected ? link.setAttribute('aria-current', 'page') : link.removeAttribute('aria-current');
  }
  main.innerHTML = '<div class="page">' + loading() + '</div>';
  const root = document.createElement('div');
  try {
    if (!ctx.profile && hasSession()) { try { await ctx.loadProfile(); } catch (err) { if (err.status !== 401) throw err; clearSession(); updateHeader(); } }
    if (current !== generation) return;
    main.replaceChildren(root);
    if (!parts.length) { document.title = 'TicketRush — планы на хороший вечер'; await catalog(root, ctx); }
    else if (parts[0] === 'events' && parts[1]) { document.title = 'Мероприятие · TicketRush'; await eventDetail(root, ctx, parts[1]); }
    else if (['login', 'register'].includes(parts[0])) { document.title = 'Вход · TicketRush'; await authPage(root, ctx, parts[0], query); }
    else if (!ctx.profile) { location.hash = '/login?next=' + encodeURIComponent(raw); return; }
    else if (parts[0] === 'account') { document.title = 'Мои билеты · TicketRush'; await accountPage(root, ctx, query); }
    else if (parts[0] === 'bookings' && parts[1]) { document.title = 'Бронирование · TicketRush'; await bookingPage(root, ctx, parts[1]); }
    else if (parts[0] === 'workspace') {
      document.title = 'Кабинет организатора · TicketRush';
      if (parts[1] === 'events' && parts[2]) await manageEvent(root, ctx, parts[2], query);
      else if (parts[1] === 'venues') await venuesPage(root, ctx);
      else if (parts[1] === 'check-in') await checkInPage(root, ctx);
      else await workspace(root, ctx);
    } else { root.innerHTML = '<section class="page empty"><h1>Страница не найдена</h1><a class="button" href="#/">На афишу</a></section>'; }
    if (current === generation) window.scrollTo({ top: 0, behavior: 'instant' });
  } catch (err) {
    if (current !== generation) return;
    main.innerHTML = `<section class="page">${errorView(err)}</section>`;
    main.querySelector('[data-retry]').onclick = route;
  }
}
// Broken/missing poster uploads fall back to a local illustration, never a CDN.
document.addEventListener('load', event => { const img = event.target; if (img instanceof HTMLImageElement && img.src.includes('/api/v1/events/') && (img.naturalWidth < 32 || img.naturalHeight < 32)) img.src = '/app/assets/poster.svg'; }, true);
document.addEventListener('error', event => { const img = event.target; if (img instanceof HTMLImageElement && !img.src.endsWith('/app/assets/poster.svg') && !img.src.startsWith('blob:')) img.src = '/app/assets/poster.svg'; }, true);
window.addEventListener('hashchange', route);
window.addEventListener('pageshow', event => { if (event.persisted) route(); });
route();
