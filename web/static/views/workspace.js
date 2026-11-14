import { api, message } from '../api.js';
import { e, icon, datetime, date, money, badge, categories, category, poster, loading, empty, errorView, pager, bindForm, fieldsError, modal, toast, action, busy, localInput, readDate } from '../ui.js';

const organizations = ctx => ctx.profile.organizations.filter(o => o.role === 'organizer');
function selectedOrg(ctx, preferred) {
  let saved; try { saved = sessionStorage.getItem('ticketrush.organization'); } catch {}
  const list = organizations(ctx);
  return list.find(o => o.id === preferred) || list.find(o => o.id === saved) || list[0];
}
function rememberOrg(id) { try { sessionStorage.setItem('ticketrush.organization', id); } catch {} }
function shell(root, ctx, active, content) {
  root.innerHTML = `<div class="workspace-shell page"><aside class="workspace-sidebar"><span class="eyebrow">За кулисами</span><h2>Ваши события.<br><span>Ваши правила.</span></h2><nav aria-label="Рабочий кабинет"><a class="${active === 'events' ? 'active' : ''}" href="#/workspace">${icon('grid')} Мероприятия</a><a class="${active === 'venues' ? 'active' : ''}" href="#/workspace/venues">${icon('pin')} Площадки</a><a class="${active === 'check-in' ? 'active' : ''}" href="#/workspace/check-in">${icon('ticket')} Контроль входа</a></nav><div class="sidebar-note">Организуйте то,<br>что хочется посетить.<span>↗</span></div></aside><section class="workspace-main">${content}</section></div>`;
}
const categoryOptions = value => Object.entries(categories).map(([key, label]) => `<option value="${key}" ${key === value ? 'selected' : ''}>${label}</option>`).join('');
function eventFields(event = {}) {
  const custom = event.category && !categories[event.category] ? `<option value="${e(event.category)}" selected>${e(event.category)}</option>` : '';
  return `<label>Название<input name="name" value="${e(event.name)}" placeholder="Например, вечер живой музыки" required maxlength="200"></label><label>Категория<select name="category">${custom}${categoryOptions(event.category || 'conference')}</select></label><label>Описание<textarea name="description" rows="5" maxlength="10000" placeholder="Что ждёт гостей?">${e(event.description)}</textarea></label>${fieldsError()}`;
}
export async function workspace(root, ctx) {
  const org = selectedOrg(ctx);
  shell(root, ctx, 'events', `<div class="page-heading"><div><span class="eyebrow">Кабинет организатора</span><h1>Мои мероприятия</h1><p class="muted">От первой идеи до полного зала.</p></div>${org ? `<button class="button" id="new-event">${icon('plus')} Создать</button>` : ''}</div>${org ? `<label class="organization-select">Организация<select id="organization">${organizations(ctx).map(o => `<option value="${e(o.id)}" ${o.id === org.id ? 'selected' : ''}>${e(o.name)}</option>`).join('')}</select></label><div id="workspace-events">${loading()}</div><div class="pager" data-pager></div>` : empty('Давайте создадим что-то интересное', 'Для управления мероприятиями нужен доступ организатора. Если вы представляете организацию, запросите доступ у её администратора.', ctx.profile.venue_owner ? '<a class="button" href="#/workspace/venues">Мои площадки ↗</a>' : '<a class="button secondary" href="#/">А пока — на афишу ↗</a>')}`);
  if (!org) return;
  let offset = 0;
  const target = root.querySelector('#workspace-events');
  const currentOrg = () => root.querySelector('#organization').value;
  async function load() {
    target.innerHTML = loading();
    const requestedOrg = currentOrg();
    try {
      const page = await api(`/organizer/events?limit=12&offset=${offset}`, { auth: true, org: requestedOrg });
      if (!root.isConnected || currentOrg() !== requestedOrg) return;
      target.innerHTML = page.items.length ? `<div class="organizer-list">${page.items.map(event => `<a class="organizer-event" href="#/workspace/events/${e(event.id)}?org=${e(requestedOrg)}"><div class="organizer-thumbnail"><img src="${e(poster(event))}" alt=""></div><div><div class="event-labels">${badge(event.status)}<span>${e(category(event.category))}</span></div><h3>${e(event.name)}</h3><p>Обновлено ${e(date(event.updated_at, { year: 'numeric' }))}</p></div><span class="round-arrow">${icon('arrow')}</span></a>`).join('')}</div>` : empty('Начнём с вашей идеи', 'Создайте мероприятие, добавьте сеанс и тарифы, затем откройте продажи.');
      pager(root, page.items.length, 12, offset, value => { offset = value; load(); });
    } catch (err) { target.innerHTML = errorView(err); target.querySelector('[data-retry]').onclick = load; }
  }
  root.querySelector('#organization').onchange = () => { offset = 0; rememberOrg(currentOrg()); load(); };
  root.querySelector('#new-event').onclick = () => modal('Новое мероприятие', `<form id="event-create">${eventFields()}<button class="button full" type="submit">Создать черновик ${icon('arrow')}</button></form>`, dialog => {
    bindForm(dialog, 'form', async data => { const orgID = currentOrg(); const event = await api('/organizer/events', { auth: true, org: orgID, method: 'POST', data }); dialog.close(); toast('Черновик создан. Добавим сеанс?'); location.hash = `/workspace/events/${event.id}?org=${orgID}`; });
  });
  await load();
}

export async function manageEvent(root, ctx, id, query) {
  const org = selectedOrg(ctx, query.get('org'));
  if (!org) { await workspace(root, ctx); return; }
  const base = `/organizer/events/${id}`, options = { auth: true, org: org.id };
  const [event, sessionsPage, venues] = await Promise.all([api(base, options), api(base + '/sessions?limit=100&offset=0', options), api('/venues?limit=100&offset=0')]);
  const sessions = sessionsPage.items || [], venueByID = new Map(venues.items.map(v => [v.id, v]));
  const draft = event.status === 'draft';
  shell(root, ctx, 'events', `<a class="back-link" href="#/workspace">← Мои мероприятия</a><div class="page-heading manage-heading"><div>${badge(event.status)}<h1>${e(event.name)}</h1><p class="muted">${e(org.name)}</p></div><div class="heading-actions">${event.status === 'published' ? `<a class="button secondary small" href="#/events/${e(id)}">На афише ${icon('arrow')}</a>` : ''}${draft ? '<button class="button small" id="publish-event">Опубликовать ↗</button>' : ''}</div></div>${draft ? '<div class="publish-steps"><span>01 · Заполните описание</span><span>02 · Добавьте сеанс и тарифы</span><span>03 · Опубликуйте сеанс, затем мероприятие</span></div>' : ''}<div class="manage-grid"><div><section class="surface"><div class="section-heading"><h2>О мероприятии</h2></div>${draft ? `<form id="event-edit">${eventFields(event)}<button class="button secondary" type="submit">Сохранить изменения</button></form>` : `<div class="event-readonly"><span class="eyebrow">${e(category(event.category))}</span><p class="description">${e(event.description || 'Описание не заполнено.')}</p><p class="field-hint">Описание и время редактируются до публикации. Афишу можно обновить ниже.</p></div>`}</section><section class="surface"><div class="section-heading"><h2>Афиша</h2></div><p class="muted">PNG или JPEG до 5 МБ. Хорошая афиша помогает выделиться.</p><form id="poster-upload"><label class="upload-zone">${icon('plus')}<span>Выберите изображение</span><input name="poster" type="file" accept="image/png,image/jpeg" required></label>${fieldsError()}<button class="button secondary" type="submit">Загрузить афишу</button></form></section><section class="surface"><div class="section-heading"><h2>Сеансы <span class="count">${sessions.length}</span></h2>${draft ? `<button class="button small secondary" id="add-session">${icon('plus')} Добавить</button>` : ''}</div><div id="session-list">${sessions.length ? sessions.map(s => `<article class="session-card" data-session="${e(s.id)}"><div class="session-heading"><div>${badge(s.status)}<h3>${e(datetime(s.starts_at))}</h3><p>${e(venueByID.get(s.venue_id)?.name || 'Площадка')} · ${e(venueByID.get(s.venue_id)?.city || '')}</p></div>${s.status === 'draft' ? '<button class="text-button" data-edit-session>Изменить время</button>' : ''}</div><p class="field-hint">Продажи: ${e(datetime(s.sales_start_at))} — ${e(datetime(s.sales_end_at))}</p><div class="session-tariffs">${loading()}</div><div class="session-actions">${draft && s.status === 'draft' ? '<button class="button ghost small" data-add-tariff>+ Добавить тариф</button>' : ''}${s.status === 'draft' ? '<button class="button small" data-publish-session>Открыть продажи</button>' : ''}</div></article>`).join('') : '<p class="muted">Добавьте дату, площадку и время продаж, чтобы гости могли выбрать билеты.</p>'}</div></section>${draft ? '<button class="text-button danger-text" id="delete-event">Удалить черновик</button>' : ''}</div><aside><section class="surface analytics"><div class="section-heading"><h2>Как идут дела</h2></div><p class="muted">Действия с бронями за выбранный период. Обновляются с небольшой задержкой.</p><form id="report-form"><label>С<input name="from" type="date" required></label><label>По<input name="to" type="date" required></label><button class="button secondary full" type="submit">Обновить отчёт</button>${fieldsError()}</form><div id="report-results">${loading()}</div></section><div class="workspace-tip">Хороший план:<br><strong>превратить посетителей<br>в постоянных гостей.</strong><span>✳</span></div></aside></div>`);
  bindForm(root, '#event-edit', async data => { await api(base, { ...options, method: 'PATCH', data }); toast('Изменения сохранены'); await ctx.reload(); });
  bindForm(root, '#poster-upload', async (data, form) => {
    const file = form.querySelector('[type=file]').files[0];
    if (!file || !['image/png', 'image/jpeg'].includes(file.type)) { form.querySelector('.form-error').textContent = 'Выберите PNG или JPEG.'; return; }
    if (file.size > 5 * 1024 * 1024) { form.querySelector('.form-error').textContent = 'Изображение должно быть не больше 5 МБ.'; return; }
    await api(base + '/poster', { ...options, method: 'POST', data: file }); toast('Афиша загружена'); form.reset();
  });
  root.querySelector('#publish-event')?.addEventListener('click', action(async event => {
    busy(event.currentTarget, true);
    try { await api(base + '/publish', { ...options, method: 'POST' }); toast('Мероприятие теперь на афише'); await ctx.reload(); }
    catch (err) { toast(message(err, err.status === 409 ? 'Сначала добавьте тариф и опубликуйте хотя бы один сеанс с действующим периодом продаж.' : undefined), true); busy(root.querySelector('#publish-event'), false); }
  }));
  root.querySelector('#delete-event')?.addEventListener('click', () => modal('Удалить черновик?', '<p>Это действие удалит мероприятие. Черновик с добавленными сеансами удалить нельзя.</p><p class="form-error" role="alert"></p><button class="button danger full" data-delete>Удалить мероприятие</button>', dialog => {
    dialog.querySelector('[data-delete]').onclick = async click => { busy(click.currentTarget, true); try { await api(base, { ...options, method: 'DELETE' }); dialog.close(); toast('Черновик удалён'); location.hash = '/workspace'; } catch (err) { dialog.querySelector('.form-error').textContent = message(err, err.status === 409 ? 'У мероприятия есть сеансы или оно уже опубликовано.' : undefined); busy(dialog.querySelector('[data-delete]'), false); } };
  }));
  root.querySelector('#add-session')?.addEventListener('click', () => createSession(base, options, venues.items, ctx));
  for (const s of sessions) {
    const node = root.querySelector(`[data-session="${s.id}"]`);
    loadTariffs(node, base, s.id, options);
    node.querySelector('[data-add-tariff]')?.addEventListener('click', () => createTariff(base, s.id, options, ctx));
    node.querySelector('[data-edit-session]')?.addEventListener('click', () => editSession(base, s, options, ctx));
    node.querySelector('[data-publish-session]')?.addEventListener('click', action(async click => {
      const button = click.currentTarget; busy(button, true);
      try { await api(`${base}/sessions/${s.id}/publish`, { ...options, method: 'POST' }); toast('Сеанс опубликован'); await ctx.reload(); }
      catch (err) { toast(message(err, err.status === 409 ? 'Нужен хотя бы один тариф и действующие даты сеанса и продаж.' : undefined), true); busy(button, false); }
    }));
  }
  const reportForm = root.querySelector('#report-form');
  reportForm.elements.from.value = localInput(Date.now() - 6 * 86400000).slice(0, 10);
  reportForm.elements.to.value = localInput(Date.now()).slice(0, 10);
  async function loadReport() {
    const result = root.querySelector('#report-results'); result.innerHTML = loading();
    const from = new Date(reportForm.elements.from.value + 'T00:00:00'), to = new Date(reportForm.elements.to.value + 'T00:00:00'); to.setDate(to.getDate() + 1);
    if (!Number.isFinite(+from) || !Number.isFinite(+to) || from >= to || (to - from) > 90 * 86400000) { result.innerHTML = '<p class="form-error">Выберите период от 1 до 90 дней.</p>'; return; }
    try {
      const report = await api(base + '/analytics/bookings?' + new URLSearchParams({ from: from.toISOString(), to: to.toISOString() }), options);
      const rows = [['created', 'Созданы'], ['confirmed', 'Подтверждены'], ['cancelled', 'Отменены'], ['expired', 'Истекли']];
      const max = Math.max(1, ...rows.map(([key]) => Number(report[key].bookings)));
      result.innerHTML = `<div class="analytics-highlight"><span>Подтверждено билетов</span><strong>${e(report.confirmed.tickets)}</strong></div><div class="report-label"><span>Статус</span><span>Брони / билеты</span></div>${rows.map(([key, label]) => `<div class="stat-row ${key}"><div><span>${label}</span><strong>${e(report[key].bookings)} <small>/ ${e(report[key].tickets)}</small></strong></div><progress max="${max}" value="${e(report[key].bookings)}" aria-label="${label}"></progress></div>`).join('')}<p class="field-hint">Это действия за период, а не текущий остаток билетов.</p>`;
    } catch (err) { result.innerHTML = errorView(err); result.querySelector('[data-retry]').onclick = loadReport; }
  }
  bindForm(root, '#report-form', loadReport); await loadReport();
}
async function loadTariffs(node, base, sessionID, options) {
  const target = node.querySelector('.session-tariffs');
  try { const page = await api(`${base}/sessions/${sessionID}/ticket-types?limit=100&offset=0`, options); target.innerHTML = page.length ? `<div class="tariff-table">${page.map(t => `<div><span><strong>${e(t.name)}</strong><small>${e(t.available)} из ${e(t.capacity)} доступно</small></span><b>${money(t.price.amount)}</b></div>`).join('')}</div>` : '<p class="field-hint">Тарифов пока нет. Добавьте хотя бы один перед публикацией.</p>'; }
  catch (err) { target.innerHTML = errorView(err); target.querySelector('[data-retry]').onclick = () => loadTariffs(node, base, sessionID, options); }
}
function createSession(base, options, venues, ctx) {
  if (!venues.length) { modal('Сначала нужна площадка', '<p>В каталоге ещё нет площадок. Владелец площадки может добавить её в разделе «Площадки».</p>'); return; }
  const start = Date.now() + 14 * 86400000;
  modal('Добавить сеанс', `<form id="session-create"><label>Площадка<select name="venue_id">${venues.map(v => `<option value="${e(v.id)}">${e(v.city)} · ${e(v.name)}</option>`).join('')}</select></label><div class="form-grid"><label>Начало<input name="starts_at" type="datetime-local" value="${localInput(start)}" required></label><label>Окончание · необязательно<input name="ends_at" type="datetime-local" value="${localInput(start + 2 * 3600000)}"></label><label>Начало продаж<input name="sales_start_at" type="datetime-local" value="${localInput(Date.now() - 60000)}" required></label><label>Конец продаж<input name="sales_end_at" type="datetime-local" value="${localInput(start - 3600000)}" required></label></div><p class="field-hint">Время указано в часовом поясе вашего устройства.</p>${fieldsError()}<button class="button full" type="submit">Создать сеанс</button></form>`, dialog => {
    bindForm(dialog, 'form', async (data, form) => {
      if (new Date(data.sales_start_at) >= new Date(data.sales_end_at) || new Date(data.sales_end_at) > new Date(data.starts_at) || (data.ends_at && new Date(data.ends_at) <= new Date(data.starts_at))) { form.querySelector('.form-error').textContent = 'Проверьте порядок дат: продажи должны завершиться до начала сеанса, окончание — после начала.'; return; }
      await api(base + '/sessions', { ...options, method: 'POST', data: { venue_id: data.venue_id, starts_at: readDate(data.starts_at), ends_at: data.ends_at ? readDate(data.ends_at) : null, sales_start_at: readDate(data.sales_start_at), sales_end_at: readDate(data.sales_end_at) } }); dialog.close(); toast('Сеанс создан. Добавьте тарифы.'); await ctx.reload();
    });
  }, true);
}
function editSession(base, s, options, ctx) {
  modal('Изменить время сеанса', `<form><label>Начало<input name="starts_at" type="datetime-local" required value="${localInput(s.starts_at)}"></label><label>Окончание · необязательно<input name="ends_at" type="datetime-local" value="${s.ends_at ? localInput(s.ends_at) : ''}"></label><p class="field-hint">Изменить время можно только у черновика. Начало должно оставаться после конца продаж.</p>${fieldsError()}<button class="button full" type="submit">Сохранить время</button></form>`, dialog => {
    bindForm(dialog, 'form', async data => { await api(`${base}/sessions/${s.id}`, { ...options, method: 'PATCH', data: { starts_at: readDate(data.starts_at), ends_at: data.ends_at ? readDate(data.ends_at) : null } }); dialog.close(); toast('Время обновлено'); await ctx.reload(); });
  });
}
function createTariff(base, sessionID, options, ctx) {
  modal('Новый тариф', `<form><label>Название<input name="name" required maxlength="100" placeholder="Например, Стандарт"></label><div class="form-grid"><label>Цена, ₽<input name="amount" type="text" inputmode="decimal" pattern="[0-9]+([.,][0-9]{1,2})?" placeholder="1500.00" maxlength="19" required></label><label>Количество мест<input name="capacity" type="number" min="1" max="2147483647" step="1" required placeholder="100"></label></div><p class="field-hint">Цена фиксируется в брони. Тарифы добавляются до публикации мероприятия и сеанса.</p>${fieldsError()}<button class="button full" type="submit">Добавить тариф</button></form>`, dialog => {
    bindForm(dialog, 'form', async data => { await api(`${base}/sessions/${sessionID}/ticket-types`, { ...options, method: 'POST', data: { name: data.name, price: { amount: data.amount.replace(',', '.'), currency: 'RUB' }, capacity: Number(data.capacity) } }); dialog.close(); toast('Тариф добавлен'); await ctx.reload(); });
  });
}

export async function venuesPage(root, ctx) {
  shell(root, ctx, 'venues', `<div class="page-heading"><div><span class="eyebrow">Места, которые объединяют</span><h1>Площадки</h1><p class="muted">Найдите подходящее пространство для вашего события.</p></div>${ctx.profile.venue_owner ? `<button class="button" id="add-venue">${icon('plus')} Добавить</button>` : ''}</div>${!ctx.profile.venue_owner ? '<p class="notice">Новые площадки добавляют их владельцы. Доступ организатора позволяет выбирать существующие площадки для сеансов.</p>' : ''}<div id="venues-list">${loading()}</div><div class="pager" data-pager></div>`);
  let offset = 0;
  async function load() {
    const node = root.querySelector('#venues-list');
    try { const page = await api(`/venues?limit=20&offset=${offset}`); node.innerHTML = page.items.length ? `<div class="venues-grid">${page.items.map(v => `<article class="surface venue-card"><div class="venue-symbol">${icon('pin')}</div><span class="eyebrow">${e(v.city)}</span><h2>${e(v.name)}</h2><p>${e(v.address)}</p></article>`).join('')}</div>` : empty('Пока нет площадок', 'Владелец площадки может добавить её в каталог.'); pager(root, page.items.length, 20, offset, value => { offset = value; load(); }); }
    catch (err) { node.innerHTML = errorView(err); node.querySelector('[data-retry]').onclick = load; }
  }
  root.querySelector('#add-venue')?.addEventListener('click', () => modal('Новая площадка', `<form><label>Название<input name="name" required maxlength="200" placeholder="Название пространства"></label><label>Город<input name="city" required maxlength="100" placeholder="Например, Москва"></label><label>Адрес<input name="address" required maxlength="500" placeholder="Улица, дом"></label>${fieldsError()}<button class="button full" type="submit">Добавить площадку</button></form>`, dialog => { bindForm(dialog, 'form', async data => { await api('/venues', { auth: true, method: 'POST', data }); dialog.close(); toast('Площадка добавлена'); offset = 0; await load(); }); }));
  await load();
}
export async function checkInPage(root, ctx) {
  shell(root, ctx, 'check-in', `<div class="page-heading"><div><span class="eyebrow">Встречаем гостей</span><h1>Контроль входа</h1><p class="muted">Один билет — один проход.</p></div></div>${ctx.profile.organizations.length ? '<section class="surface checkin-surface"><form id="check-in"><label>Мероприятие<select name="event" required><option value="">Выберите мероприятие</option></select></label><label>Сеанс<select name="session" required disabled><option value="">Сначала выберите мероприятие</option></select></label><label>Код с билета<textarea name="token" rows="3" required maxlength="4096" placeholder="Вставьте код, полученный из QR билета"></textarea></label><p class="field-hint">Для учебной проверки скопируйте код из окна QR покупателя. Доступ к сеансу и действительность билета проверяются при каждом проходе.</p><div id="check-in-result" aria-live="polite"></div>'+ fieldsError() +'<button class="button full" type="submit">Проверить и пропустить →</button></form></section>' : empty('Нужен доступ сотрудника', 'Проверять билеты могут организаторы и контролёры своей организации.')}`);
  if (!ctx.profile.organizations.length) return;
  const form = root.querySelector('form');
  const events = await api('/events?limit=100&offset=0');
  const orgIDs = new Set(ctx.profile.organizations.map(o => o.id));
  form.elements.event.innerHTML += events.items.filter(ev => orgIDs.has(ev.organizer_id)).map(ev => `<option value="${e(ev.id)}">${e(ev.name)}</option>`).join('');
  form.elements.event.onchange = action(async () => {
    const select = form.elements.session, id = form.elements.event.value; select.disabled = true; select.innerHTML = '<option value="">Выберите сеанс</option>';
    if (!id) return;
    const page = await api(`/events/${id}/sessions?limit=100&offset=0`);
    if (form.elements.event.value !== id) return;
    select.innerHTML += page.items.map(s => `<option value="${e(s.id)}">${e(datetime(s.starts_at))}</option>`).join(''); select.disabled = false;
  });
  bindForm(root, '#check-in', async (data, form) => {
    const result = root.querySelector('#check-in-result');
    try { const ticket = await api(`/controller/sessions/${data.session}/check-ins`, { auth: true, method: 'POST', data: { token: data.token.trim() } }); result.innerHTML = `<div class="checkin-success">${icon('check')}<strong>Добро пожаловать!</strong><p>Билет № ${e(ticket.id.slice(0, 8))} отмечен использованным.</p></div>`; form.elements.token.value = ''; form.elements.token.focus(); }
    catch (err) { result.innerHTML = `<p class="notice error" role="alert">${e(message(err, err.status === 409 ? 'Проход запрещён: билет уже использован или больше не действует.' : err.status === 404 ? 'Билет не найден для выбранного сеанса или у вас нет доступа.' : undefined))}</p>`; }
  });
}
