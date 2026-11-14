import { api, message } from '../api.js';
import { e, icon, date, datetime, money, cents, quantityMoney, category, categories, poster, loading, empty, errorView, pager, action, busy } from '../ui.js';

export async function catalog(root, ctx) {
  let offset = 0, items = [], query = '', selected = '';
  root.innerHTML = `<section class="hero page"><div class="eyebrow"><span class="live-dot"></span> Время для новых впечатлений</div><h1>Хороший вечер<br>начинается <span>здесь.</span><svg class="hero-star" viewBox="0 0 100 100" aria-hidden="true"><path d="m50 0 11 32 29-17-17 29 27 6-27 10 17 27-29-17L50 100 39 70 10 87l17-27L0 50l27-6L10 15l29 17z" fill="currentColor"/></svg></h1><div class="hero-bottom"><p>Выбирайте то, что откликается.<br>А о билетах позаботимся мы.</p><span class="hero-note">Ваш следующий<br>«это было круто» ↗</span></div></section>
  <section class="page catalog-section"><div class="section-heading"><div><span class="eyebrow">Найти свой повод выйти</span><h2>На афише</h2></div><label class="search">${icon('search')}<input type="search" id="catalog-search" placeholder="Название мероприятия" aria-label="Поиск среди мероприятий на странице"></label></div><div class="filter-row"><div class="chips" id="categories"><button class="chip active" data-category="">Всё интересное</button>${Object.entries(categories).map(([key, text]) => `<button class="chip" data-category="${key}">${text}</button>`).join('')}</div><span class="filter-note">Поиск и категории — на текущей странице</span></div><div id="catalog-results">${loading()}</div><div class="pager" data-pager></div></section>
  <section class="page"><div class="how-banner"><div class="how-title">Планы есть.<br><span>Билет тоже.</span></div><div><span class="step-number">01</span><h3>Выберите впечатление</h3><p>Мероприятие, удобный сеанс<br>и подходящий тариф.</p></div><div><span class="step-number">02</span><h3>Забронируйте билет</h3><p>Места будут ждать вас<br>до конца времени брони.</p></div><div><span class="step-number">03</span><h3>Покажите QR на входе</h3><p>Ваши билеты всегда рядом —<br>в личном кабинете.</p></div></div></section>`;
  const results = root.querySelector('#catalog-results');
  function draw() {
    const visible = items.filter(item => (!selected || item.category === selected) && (!query || (item.name + ' ' + item.description).toLocaleLowerCase('ru').includes(query)));
    results.innerHTML = visible.length ? `<div class="event-grid">${visible.map((item, i) => `<a class="event-card tone-${i % 3}" href="#/events/${e(item.id)}"><div class="card-poster"><img src="${e(poster(item))}" alt="" loading="lazy"><span class="card-category">${e(category(item.category))}</span><span class="card-arrow">${icon('arrow')}</span></div><div class="card-content"><div class="card-meta" data-meta="${e(item.id)}">${icon('calendar')} Выбрать сеанс</div><h3>${e(item.name)}</h3><p class="card-description">${e(item.description || 'Новые впечатления ждут вас.')}</p><div class="card-bottom"><span data-place="${e(item.id)}">Открыть мероприятие</span><span class="card-price" data-price="${e(item.id)}">Билеты ${icon('arrow')}</span></div></div></a>`).join('')}</div>` : empty('Пока ничего не нашлось', query || selected ? 'Попробуйте другое название или категорию.' : 'Организаторы ещё готовят мероприятия. Загляните немного позже.');
    for (const item of visible) enrich(item);
    pager(root, items.length, 12, offset, value => { offset = value; load(); });
  }
  async function enrich(item) {
    try {
      const page = await api(`/events/${item.id}/sessions?limit=1&offset=0`);
      const session = page.items?.[0]; if (!session || !root.isConnected) return;
      const node = results.querySelector(`[data-meta="${item.id}"]`); if (node) node.innerHTML = `${icon('calendar')}${e(datetime(session.starts_at))}`;
      const [venue, types] = await Promise.allSettled([api(`/venues/${session.venue_id}`), api(`/events/${item.id}/sessions/${session.id}/ticket-types?limit=100&offset=0`)]);
      const place = results.querySelector(`[data-place="${item.id}"]`);
      if (place && venue.status === 'fulfilled') place.textContent = venue.value.city + ' · ' + venue.value.name;
      const prices = types.status === 'fulfilled' ? (types.value.items || []).filter(t => t.available > 0).map(t => t.price.amount) : [];
      const price = results.querySelector(`[data-price="${item.id}"]`); if (price && prices.length) price.textContent = 'от ' + money(prices.reduce((a, b) => cents(a) < cents(b) ? a : b));
    } catch { /* The detail page shows authoritative availability. */ }
  }
  async function load() {
    results.innerHTML = loading();
    try { const page = await api(`/events?limit=12&offset=${offset}`); if (!root.isConnected) return; items = page.items || []; draw(); }
    catch (err) { results.innerHTML = errorView(err); results.querySelector('[data-retry]').onclick = load; }
  }
  root.querySelector('#catalog-search').oninput = event => { query = event.target.value.trim().toLocaleLowerCase('ru'); draw(); };
  root.querySelector('#categories').onclick = event => { const btn = event.target.closest('[data-category]'); if (!btn) return; selected = btn.dataset.category; for (const b of root.querySelectorAll('.chip')) b.classList.toggle('active', b === btn); draw(); };
  await load();
}

export async function eventDetail(root, ctx, id) {
  const [event, page] = await Promise.all([api(`/events/${id}`), api(`/events/${id}/sessions?limit=100&offset=0`)]);
  const sessions = page.items || [];
  root.innerHTML = `<div class="page detail-page"><a class="back-link" href="#/">← Все мероприятия</a><div class="event-detail"><div><div class="detail-poster"><img src="${e(poster(event))}" alt="Афиша ${e(event.name)}"></div><div class="event-info"><span class="eyebrow">${e(category(event.category))}</span><h1>${e(event.name)}</h1><p class="description">${e(event.description || 'Выберите удобный сеанс и приходите за впечатлениями.')}</p></div></div><aside class="booking-panel"><span class="eyebrow">Ваш повод выйти</span><h2>Выберите билеты</h2>${sessions.length ? `<label>Сеанс<select id="session-choice">${sessions.map(s => `<option value="${e(s.id)}">${e(datetime(s.starts_at))}</option>`).join('')}</select></label><div id="venue-info" class="venue-line"></div><div id="tariffs">${loading()}</div>` : empty('Сеансов пока нет', 'Продажи ещё не начались или уже завершились.')}</aside></div></div>`;
  const target = root.querySelector('#tariffs');
  async function loadSession() {
    const session = sessions.find(s => s.id === root.querySelector('#session-choice').value);
    target.innerHTML = loading();
    try {
      const [types, venue] = await Promise.all([api(`/events/${id}/sessions/${session.id}/ticket-types?limit=100&offset=0`), api(`/venues/${session.venue_id}`)]);
      if (!root.isConnected || root.querySelector('#session-choice').value !== session.id) return;
      root.querySelector('#venue-info').innerHTML = `${icon('pin')}<span><strong>${e(venue.name)}</strong><br>${e(venue.city)}, ${e(venue.address)}</span>`;
      const tariffs = types.items || [];
      const now = Date.now(), starts = new Date(session.sales_start_at).getTime(), ends = new Date(session.sales_end_at).getTime();
      const open = now >= starts && now < ends;
      const available = tariffs.find(t => t.available > 0);
      target.innerHTML = `<form id="booking-form"><div class="tariff-list">${tariffs.map(t => `<label class="tariff ${t.available ? '' : 'sold-out'}"><input type="radio" name="tariff" value="${e(t.id)}" ${available?.id === t.id ? 'checked' : ''} ${t.available ? '' : 'disabled'}><span><strong>${e(t.name)}</strong><small>${t.available ? `Осталось ${e(t.available)}` : 'Билетов нет'}</small></span><b>${money(t.price.amount)}</b></label>`).join('') || '<p>Тарифы пока не опубликованы.</p>'}</div><div class="quantity-row"><label for="quantity">Количество</label><div class="quantity"><button type="button" data-minus aria-label="Уменьшить количество">−</button><input type="number" id="quantity" min="1" max="10" value="1" required aria-label="Количество билетов"><button type="button" data-plus aria-label="Увеличить количество">+</button></div></div><div class="total-row"><span>Итого</span><strong id="booking-total">—</strong></div>${!open ? `<p class="notice">${now < starts ? 'Продажи начнутся ' + e(datetime(session.sales_start_at)) : 'Продажи завершены'}</p>` : ''}<button class="button full" type="submit" ${!open || !available ? 'disabled' : ''}>${ctx.profile ? 'Забронировать' : 'Войти и забронировать'} ${icon('arrow')}</button><p class="panel-note">Сначала бронь, затем подтверждение.<br>Demo-оплата: реальные деньги не списываются.</p><p class="form-error" role="alert"></p></form>`;
      const form = target.querySelector('form'), qty = form.querySelector('#quantity');
      const chosen = () => tariffs.find(t => t.id === new FormData(form).get('tariff'));
      const update = () => { const t = chosen(); qty.max = Math.max(1, Math.min(10, t?.available || 1)); qty.value = Math.max(1, Math.min(Math.trunc(Number(qty.value) || 1), Number(qty.max))); form.querySelector('#booking-total').textContent = t ? quantityMoney(t.price.amount, Number(qty.value)) : '—'; };
      form.onchange = update; qty.oninput = update;
      form.querySelector('[data-minus]').onclick = () => { qty.value = Math.max(1, Number(qty.value) - 1); update(); };
      form.querySelector('[data-plus]').onclick = () => { qty.value = Math.min(Number(qty.max), Number(qty.value) + 1); update(); };
      update();
      form.onsubmit = async event => {
        event.preventDefault();
        if (!ctx.profile) { location.hash = '/login?next=' + encodeURIComponent('/events/' + id); return; }
        const button = form.querySelector('[type=submit]'), error = form.querySelector('.form-error'); error.textContent = ''; busy(button, true);
        try { const b = await api(`/events/${id}/sessions/${session.id}/bookings`, { auth: true, method: 'POST', data: { ticket_type_id: chosen().id, quantity: Number(qty.value) } }); location.hash = `/bookings/${b.id}`; }
        catch (err) { error.textContent = message(err, err.status === 409 ? 'Билеты закончились или продажи завершены. Выберите другой тариф.' : err.status === 0 ? 'Ответ не получен. Проверьте «Мои брони» перед повторной попыткой.' : undefined); }
        finally { busy(button, false); }
      };
    } catch (err) { target.innerHTML = errorView(err); target.querySelector('[data-retry]').onclick = loadSession; }
  }
  if (sessions.length) { root.querySelector('#session-choice').onchange = loadSession; await loadSession(); }
}
