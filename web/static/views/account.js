import { api, setSession, message } from '../api.js';
import { e, icon, date, datetime, money, itemsMoney, badge, loading, empty, errorView, pager, bindForm, fieldsError, modal, toast, busy, action } from '../ui.js';

export async function authPage(root, ctx, mode, query) {
  const register = mode === 'register';
  const requested = query.get('next') || '/account';
  const next = requested.startsWith('/') && !requested.startsWith('//') && !/^\/(login|register)/.test(requested) ? requested : '/account';
  if (ctx.profile) { location.hash = next; return; }
  root.innerHTML = `<section class="page auth-layout"><div class="auth-art"><div class="eyebrow">Билеты к вашим историям</div><h1>Больше<br>«я там<br><span>был».</span></h1><div class="art-ticket">${icon('ticket')}<span>ONE GOOD EVENING<br><strong>Много впечатлений.</strong></span>${icon('arrow')}</div><p>Бронируйте билеты, сохраняйте QR<br>и собирайте хорошие воспоминания.</p></div><div class="auth-form-wrap"><a class="back-link" href="#/">← К афише</a><h2>${register ? 'Давайте знакомиться' : 'С возвращением'}</h2><p class="muted">${register ? 'Один аккаунт для всех ваших впечатлений.' : 'Ваш следующий хороший вечер уже рядом.'}</p><form id="auth-form"><label>Email<input type="email" name="email" autocomplete="email" placeholder="you@example.com" required maxlength="254"></label><label>Пароль<div class="password-field"><input type="password" name="password" autocomplete="${register ? 'new-password' : 'current-password'}" placeholder="${register ? 'От 15 символов' : 'Ваш пароль'}" ${register ? 'minlength="15"' : ''} maxlength="128" required><button type="button" class="text-button" data-show>Показать</button></div></label>${register ? '<p class="field-hint">От 15 до 128 символов. Можно использовать длинную запоминающуюся фразу.</p>' : ''}${fieldsError()}<button class="button full" type="submit">${register ? 'Создать аккаунт' : 'Войти'} ${icon('arrow')}</button></form><p class="auth-switch">${register ? 'Уже с нами?' : 'Пока нет аккаунта?'} <a href="#/${register ? 'login' : 'register'}?next=${e(encodeURIComponent(next))}">${register ? 'Войти' : 'Зарегистрироваться'}</a></p><p class="auth-footnote">Подтверждение брони в этом приложении работает в demo-режиме, без списания денег.</p></div></section>`;
  const password = root.querySelector('[name=password]');
  root.querySelector('[data-show]').onclick = event => { const hidden = password.type === 'password'; password.type = hidden ? 'text' : 'password'; event.currentTarget.textContent = hidden ? 'Скрыть' : 'Показать'; };
  bindForm(root, '#auth-form', async (data, form) => {
    try {
      if (register) await api('/auth/register', { method: 'POST', data });
      const result = await api('/auth/login', { method: 'POST', data });
      setSession(result); await ctx.loadProfile(); toast(register ? 'Рады знакомству!' : 'Вы вошли в аккаунт'); location.hash = next;
    } catch (err) {
      form.querySelector('.form-error').textContent = message(err, err.status === 401 ? 'Неверный email или пароль.' : register && err.status === 409 ? 'Этот email уже зарегистрирован. Попробуйте войти.' : undefined);
      if (register && err.status === 429) form.querySelector('.form-error').textContent += ' Если аккаунт уже создан, перейдите ко входу.';
    }
  });
}

function accountHeading(profile, active) {
  return `<div class="page-heading"><div><span class="eyebrow">Ваш личный кабинет</span><h1>Впечатления — с вами.</h1><p class="muted">${e(profile.email)}</p></div><a class="button secondary" href="#/">На афишу ${icon('arrow')}</a></div><nav class="tabs" aria-label="Кабинет"><a class="${active === 'tickets' ? 'active' : ''}" href="#/account">${icon('ticket')} Мои билеты</a><a class="${active === 'bookings' ? 'active' : ''}" href="#/account?tab=bookings">${icon('clock')} Мои брони</a></nav>`;
}
export async function accountPage(root, ctx, query) {
  const tab = query.get('tab') === 'bookings' ? 'bookings' : 'tickets';
  root.innerHTML = `<section class="page cabinet">${accountHeading(ctx.profile, tab)}<div id="account-results">${loading()}</div><div class="pager" data-pager></div></section>`;
  const result = root.querySelector('#account-results'); let offset = 0;
  async function load() {
    result.innerHTML = loading();
    try {
      if (tab === 'bookings') {
        const page = await api(`/me/bookings?limit=12&offset=${offset}`, { auth: true });
        result.innerHTML = page.items.length ? `<div class="booking-list">${page.items.map(b => `<a class="booking-row" href="#/bookings/${e(b.id)}"><div class="booking-symbol">${icon('ticket')}</div><div class="booking-name"><h3>${e(b.event_name)}</h3><p>${e(datetime(b.starts_at))} · ${e(b.venue_name)}</p><small>${e(b.quantity)} бил. · № ${e(b.id.slice(0, 8))}</small></div><div class="booking-state">${badge(b.status)}<strong>${money(b.total)}</strong></div>${icon('chevron')}</a>`).join('')}</div>` : empty('Сначала найдём повод выйти', 'Здесь появятся ваши брони, в том числе отменённые и истёкшие.', '<a class="button" href="#/">Выбрать мероприятие ↗</a>');
        pager(root, page.items.length, 12, offset, value => { offset = value; load(); });
      } else {
        const [page, bookings] = await Promise.all([api(`/me/tickets?limit=12&offset=${offset}`, { auth: true }), api('/me/bookings?limit=100&offset=0', { auth: true })]);
        const byID = new Map(bookings.items.map(b => [b.id, b]));
        result.innerHTML = page.items.length ? `<div class="tickets-grid">${page.items.map(t => {
          const b = byID.get(t.booking_id);
          return `<article class="ticket-card"><div class="ticket-top"><span class="eyebrow">Ваш билет</span>${badge(t.status)}</div><h2>${e(b?.event_name || 'Билет на мероприятие')}</h2><p>${b ? e(datetime(b.starts_at)) : e(date(t.created_at))}</p><div class="ticket-location">${icon('pin')} ${e(b ? b.city + ' · ' + b.venue_name : 'Подробности в брони')}</div><div class="ticket-perforation"></div><div class="ticket-bottom"><span>№ ${e(t.id.slice(0, 8).toUpperCase())}</span><button class="button ${t.status === 'active' ? '' : 'secondary'} small" data-ticket="${e(t.id)}">${t.status === 'active' ? 'Показать QR' : 'Подробнее'} ${icon('arrow')}</button></div></article>`;
        }).join('')}</div>` : empty('Впереди — ваши лучшие вечера', 'После подтверждения брони билеты с QR появятся здесь.', '<a class="button" href="#/">Найти мероприятие ↗</a>');
        for (const btn of result.querySelectorAll('[data-ticket]')) btn.addEventListener('click', action(() => showTicket(btn.dataset.ticket, byID, ctx)));
        pager(root, page.items.length, 12, offset, value => { offset = value; load(); });
      }
    } catch (err) { result.innerHTML = errorView(err); result.querySelector('[data-retry]').onclick = load; }
  }
  await load();
}
async function showTicket(id, bookings, ctx) {
  const ticket = await api(`/tickets/${id}`, { auth: true }), b = bookings.get(ticket.booking_id);
  const dialog = modal('Билет для вашего вечера', `<div class="qr-ticket"><span class="eyebrow">${e(b?.venue_name || 'TicketRush')}</span><h2>${e(b?.event_name || 'Ваш билет')}</h2><p>${b ? e(datetime(b.starts_at)) : ''}</p>${badge(ticket.status)}<div id="qr-image">${ticket.status === 'active' ? loading() : '<p class="notice">Этот билет больше нельзя использовать для входа.</p>'}</div><p class="ticket-id">№ ${e(ticket.id)}</p><a class="button secondary full" href="#/bookings/${e(ticket.booking_id)}" data-booking-link>Открыть бронь ${icon('arrow')}</a>${ticket.status === 'active' ? '<div class="qr-actions"><button class="text-button" data-download>Скачать QR</button><button class="text-button" data-copy>Скопировать код для входа</button></div><p class="field-hint">Покажите QR на входе. Не пересылайте его посторонним — он действителен для одного прохода.</p>' : ''}</div>`);
  dialog.querySelector('[data-booking-link]').onclick = () => dialog.close();
  if (ticket.status !== 'active') return;
  let url;
  const cleanup = () => { if (url) URL.revokeObjectURL(url); dialog.close(); };
  ctx.cleanup.push(cleanup);
  dialog.addEventListener('close', () => { if (url) URL.revokeObjectURL(url); });
  try {
    const blob = await api(`/tickets/${id}/qr`, { auth: true, blob: true });
    if (!dialog.isConnected) return;
    url = URL.createObjectURL(blob); dialog.querySelector('#qr-image').innerHTML = `<img class="qr-image" src="${e(url)}" alt="QR-код билета для входа">`;
    dialog.querySelector('[data-download]').onclick = () => { const a = document.createElement('a'); a.href = url; a.download = `ticketrush-${id}.png`; a.click(); };
    dialog.querySelector('[data-copy]').onclick = action(async () => { await navigator.clipboard.writeText(ticket.qr_payload); toast('Код скопирован'); });
  } catch (err) { dialog.querySelector('#qr-image').innerHTML = errorView(err); dialog.querySelector('[data-retry]').onclick = () => { dialog.close(); showTicket(id, bookings, ctx).catch(error => toast(message(error), true)); }; }
}

export async function bookingPage(root, ctx, id) {
  const [booking, page] = await Promise.all([api(`/bookings/${id}`, { auth: true }), api('/me/bookings?limit=100&offset=0', { auth: true })]);
  const summary = page.items.find(b => b.id === id), held = booking.status === 'held';
  root.innerHTML = `<section class="page checkout-page"><a class="back-link" href="#/account?tab=bookings">← Мои брони</a><div class="checkout-grid"><div><span class="eyebrow">${held ? 'Осталось совсем немного' : 'Ваше бронирование'}</span><h1>${held ? 'Билет почти ваш.' : booking.status === 'confirmed' ? 'У вас есть планы!' : 'Информация о брони'}</h1><p class="muted">${held ? 'Подтвердите бронь, пока места закреплены за вами.' : 'Статус и состав вашего бронирования.'}</p><div class="checkout-event"><div class="booking-symbol">${icon('ticket')}</div><div><h2>${e(summary?.event_name || 'Ваше мероприятие')}</h2>${summary ? `<p>${e(datetime(summary.starts_at))}<br>${e(summary.city)}, ${e(summary.venue_name)}</p>` : ''}</div></div><div class="notice">${held ? 'Это учебный сервис: подтверждение имитирует успешную оплату. Вводить банковские данные не нужно.' : booking.status === 'confirmed' ? 'Билеты уже в личном кабинете. Откройте QR перед входом.' : 'Если бронь отменена или истекла, места возвращаются в продажу. Можно оформить новую.'}</div></div><aside class="checkout-summary"><div class="section-heading"><h2>Ваша бронь</h2>${badge(booking.status)}</div><small class="muted">№ ${e(id.slice(0, 8))}</small><div class="summary-items">${booking.items.map(item => `<div><span>${e(item.quantity)} × билет</span><strong>${money(item.unit_price.amount)}</strong></div>`).join('')}</div><div class="total-row"><span>Итого</span><strong>${itemsMoney(booking.items)}</strong></div>${held ? `<div class="countdown">${icon('clock')}<span>Места ждут вас ещё <strong id="countdown">…</strong></span></div><button class="button full" id="confirm-booking">Подтвердить · demo ${icon('arrow')}</button><button class="text-button full" id="cancel-booking">Отменить бронь</button><p class="form-error" role="alert"></p>` : booking.status === 'confirmed' ? '<a class="button full" href="#/account">Открыть мои билеты ↗</a>' : '<a class="button full" href="#/">Вернуться на афишу ↗</a>'}</aside></div></section>`;
  if (!held) return;
  let refreshed = false;
  const updateTimer = () => {
    const seconds = Math.max(0, Math.ceil((new Date(booking.expires_at).getTime() - Date.now()) / 1000));
    root.querySelector('#countdown').textContent = `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`;
    if (!seconds) { root.querySelector('#confirm-booking').disabled = true; root.querySelector('.form-error').textContent = 'Время удержания истекло. Обновите бронь или выберите билеты заново.'; if (!refreshed) { refreshed = true; api(`/bookings/${id}`, { auth: true }).then(b => { if (b.status !== 'held' && root.isConnected) ctx.reload(); }).catch(() => {}); } }
  };
  updateTimer(); const timer = setInterval(updateTimer, 1000); ctx.cleanup.push(() => clearInterval(timer));
  root.querySelector('#confirm-booking').onclick = async event => {
    const button = event.currentTarget; busy(button, true);
    try { await api(`/bookings/${id}/confirm`, { auth: true, method: 'POST' }); toast('Бронь подтверждена. Билеты уже в кабинете!'); location.hash = '/account'; }
    catch (err) { root.querySelector('.form-error').textContent = message(err, err.status === 403 ? 'Demo-подтверждение сейчас отключено на сервере.' : err.status === 409 ? 'Бронь истекла или сеанс недоступен. Обновите страницу, чтобы проверить статус.' : undefined); }
    finally { busy(button, false); updateTimer(); }
  };
  root.querySelector('#cancel-booking').onclick = () => modal('Отменить бронирование?', '<p>Места вернутся в продажу. Если захотите прийти, нужно будет оформить новую бронь.</p><p class="form-error" role="alert"></p><button class="button danger full" data-confirm-cancel>Да, отменить бронь</button>', dialog => {
    dialog.querySelector('[data-confirm-cancel]').onclick = async event => {
      busy(event.currentTarget, true);
      try { await api(`/bookings/${id}/cancel`, { auth: true, method: 'POST' }); dialog.close(); toast('Бронь отменена'); await ctx.reload(); }
      catch (err) { dialog.querySelector('.form-error').textContent = message(err); busy(dialog.querySelector('[data-confirm-cancel]'), false); }
    };
  });
}
