import { message } from './api.js';
export const e = value => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
const paths = {
  arrow: 'M7 17 17 7M7 7h10v10', chevron: 'm9 5 7 7-7 7', search: 'm21 21-4.4-4.4M19 10.5a8.5 8.5 0 1 1-17 0 8.5 8.5 0 0 1 17 0',
  pin: 'M20 10c0 6-8 12-8 12S4 16 4 10a8 8 0 0 1 16 0ZM15 10a3 3 0 1 1-6 0 3 3 0 0 1 6 0',
  calendar: 'M8 2v4m8-4v4M3 10h18M5 4h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2',
  ticket: 'M3 7a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v3a2 2 0 0 0 0 4v3a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-3a2 2 0 0 0 0-4ZM15 5v3m0 3v2m0 3v3',
  clock: 'M12 8v5l3 2M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0', plus: 'M12 5v14M5 12h14', check: 'm5 12 4 4L19 6', close: 'm6 6 12 12M6 18 18 6',
  user: 'M20 21v-2a7 7 0 0 0-14 0v2M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0', grid: 'M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z',
};
export const icon = (name, cls = '') => `<svg class="icon ${cls}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="${paths[name] || paths.arrow}"/></svg>`;
export const date = (value, options = {}) => value ? new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long', ...options }).format(new Date(value)) : 'Дата уточняется';
export const datetime = value => date(value, { year: 'numeric', hour: '2-digit', minute: '2-digit' });
// Display decimal strings without routing rubles through binary floating point.
export function cents(value) {
  const [whole, fraction = ''] = String(value).split('.');
  return BigInt(whole || '0') * 100n + BigInt(fraction.padEnd(2, '0').slice(0, 2));
}
export function formatCents(value) {
  const whole = value / 100n, fraction = String(value % 100n).padStart(2, '0');
  return new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', minimumFractionDigits: 2, maximumFractionDigits: 2 }).formatToParts(whole).map(part => part.type === 'fraction' ? fraction : part.value).join('');
}
export const money = value => formatCents(cents(value));
export const quantityMoney = (value, quantity) => formatCents(cents(value) * BigInt(quantity));
export const itemsMoney = items => formatCents(items.reduce((sum, item) => sum + cents(item.unit_price.amount) * BigInt(item.quantity), 0n));
export const categories = { conference: 'Конференции', concert: 'Концерты', theater: 'Театр', exhibition: 'Выставки', festival: 'Фестивали', workshop: 'Мастер-классы', other: 'Другое' };
export const category = value => categories[value] || value || 'Мероприятие';
export const statusNames = { draft: 'Черновик', published: 'Опубликовано', scheduled: 'В продаже', archived: 'В архиве', held: 'Ожидает подтверждения', confirmed: 'Подтверждено', cancelled: 'Отменено', expired: 'Время истекло', active: 'Действителен', used: 'Использован', revoked: 'Аннулирован', completed: 'Завершён' };
export const badge = status => `<span class="badge status-${e(status)}">${e(statusNames[status] || status)}</span>`;
export const link = (href, label, cls = 'button') => `<a class="${cls}" href="${e(href)}">${label}</a>`;
export const poster = event => /^\/api\/v1\/events\/[a-f0-9-]+\/poster$/.test(event.poster_url || '') ? event.poster_url : '/app/assets/poster.svg';
export function empty(title, detail, action = '') { return `<div class="empty">${icon('ticket')}<h2>${e(title)}</h2><p>${e(detail)}</p>${action}</div>`; }
export function errorView(error, detail) { return `<div class="notice error" role="alert"><strong>${e(message(error, detail))}</strong>${error.requestID ? `<small>Код обращения: ${e(error.requestID)}</small>` : ''}<button class="button ghost small" data-retry>Попробовать снова</button></div>`; }
export function loading() { return '<div class="loading" role="status"><span class="spinner"></span>Загружаем…</div>'; }
export function toast(text, bad = false) {
  const node = document.createElement('div'); node.className = 'toast' + (bad ? ' bad' : ''); node.textContent = text;
  document.querySelector('#toasts').append(node); setTimeout(() => node.remove(), 5500);
}
export function action(fn) { return async event => { try { await fn(event); } catch (error) { toast(message(error), true); } }; }
export function busy(button, value) { if (!button) return; button.disabled = value; button.setAttribute('aria-busy', String(value)); }
export function bindForm(root, selector, callback) {
  root.querySelector(selector)?.addEventListener('submit', async event => {
    event.preventDefault(); const form = event.currentTarget; const button = event.submitter || form.querySelector('[type=submit]');
    const errorNode = form.querySelector('.form-error'); if (errorNode) errorNode.textContent = '';
    busy(button, true);
    try { await callback(Object.fromEntries(new FormData(form)), form); }
    catch (error) { if (errorNode) errorNode.textContent = message(error); else toast(message(error), true); }
    finally { busy(button, false); }
  });
}
export function modal(title, content, callback, wide = false) {
  const prior = document.activeElement;
  const dialog = document.createElement('dialog'); dialog.setAttribute('aria-label', title); dialog.className = 'dialog' + (wide ? ' wide' : '');
  dialog.innerHTML = `<div class="dialog-heading"><h2>${e(title)}</h2><button type="button" class="icon-button" data-close aria-label="Закрыть">${icon('close')}</button></div>${content}`;
  dialog.querySelector('[data-close]').onclick = () => dialog.close();
  dialog.addEventListener('click', event => { if (event.target === dialog) { const b = dialog.getBoundingClientRect(); if (event.clientX < b.left || event.clientX > b.right || event.clientY < b.top || event.clientY > b.bottom) dialog.close(); } });
  dialog.addEventListener('close', () => { dialog.remove(); if (prior?.isConnected) prior.focus(); });
  document.body.append(dialog); dialog.showModal(); callback?.(dialog); return dialog;
}
export function fieldsError() { return '<p class="form-error" role="alert"></p>'; }
export function pager(root, count, limit, offset, change) {
  const node = root.querySelector('[data-pager]'); if (!node) return;
  node.innerHTML = (offset || count >= limit) ? `<button class="button ghost small" data-prev ${offset ? '' : 'disabled'}>← Назад</button><span>Страница ${Math.floor(offset / limit) + 1}</span><button class="button ghost small" data-next ${count < limit ? 'disabled' : ''}>Дальше →</button>` : '';
  node.querySelector('[data-prev]')?.addEventListener('click', () => change(Math.max(0, offset - limit)));
  node.querySelector('[data-next]')?.addEventListener('click', () => change(offset + limit));
}
export const localInput = value => { const d = new Date(value); return new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16); };
export function readDate(value) { const d = new Date(value); if (!value || !Number.isFinite(d.getTime())) throw new Error('Invalid date'); return d.toISOString(); }
