#!/usr/bin/env python3
"""Run the public buyer/organizer scenario against an already running local API.
Creates a demo buyer and purchases two demo tickets; never prints auth/QR tokens.
"""
import datetime as dt
import json
import os
import struct
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
import zlib

base = os.environ.get('TICKETRUSH_SMOKE_URL', 'http://127.0.0.1:8080')
event = '55555555-5555-4555-8555-555555555555'
session = '66666666-6666-4666-8666-666666666666'
tariff = '77777777-7777-4777-8777-777777777777'
org = '33333333-3333-4333-8333-333333333333'


def request(method, path, data=None, token=None, organizer=False, status=200, raw=False):
    headers = {}
    if data is not None:
        headers['Content-Type'] = 'image/png' if isinstance(data, bytes) else 'application/json'
        data = data if isinstance(data, bytes) else json.dumps(data).encode()
    if token:
        headers['Authorization'] = 'Bearer ' + token
    if organizer:
        headers['X-Organizer-ID'] = org
    req = urllib.request.Request(base + path, data=data, headers=headers, method=method)
    try:
        res = urllib.request.urlopen(req, timeout=15)
    except urllib.error.HTTPError as err:
        res = err
    with res:
        body = res.read()
        if res.code != status:
            raise RuntimeError(f'{method} {path}: expected {status}, got {res.code}: {body.decode(errors="replace")}')
        return body if raw else json.loads(body)


email = 'smoke-' + uuid.uuid4().hex + '@example.test'
password = 'TicketRush-smoke-password-2026!'
request('POST', '/api/v1/auth/register', {'email': email, 'password': password}, status=201)
buyer = request('POST', '/api/v1/auth/login', {'email': email, 'password': password})['access_token']
organizer = request('POST', '/api/v1/auth/login', {'email': 'organizer@example.test', 'password': 'TicketRush-demo-2026!'})['access_token']
owner = request('POST', '/api/v1/auth/login', {'email': 'venue-owner@example.test', 'password': 'TicketRush-demo-2026!'})['access_token']
request('POST', '/api/v1/venues', {'name': 'Smoke venue', 'city': 'Moscow', 'address': 'Smoke address'}, token=organizer, status=403)
request('POST', '/api/v1/venues', {'name': 'Smoke venue', 'city': 'Moscow', 'address': 'Smoke address'}, token=owner, status=201)
print('OK: venue owner permission is independent of organizer membership')
now = dt.datetime.now(dt.timezone.utc)
query = urllib.parse.urlencode({'from': (now-dt.timedelta(days=1)).isoformat(), 'to': (now+dt.timedelta(days=1)).isoformat()})
report_path = f'/api/v1/organizer/events/{event}/analytics/bookings?{query}'
before = request('GET', report_path, token=organizer, organizer=True)['confirmed']['bookings']
booking = request('POST', f'/api/v1/events/{event}/sessions/{session}/bookings', {'ticket_type_id': tariff, 'quantity': 2}, token=buyer, status=201)
for _ in range(2):
    confirmation = request('POST', f'/api/v1/bookings/{booking["id"]}/confirm', token=buyer)
    assert confirmation['status'] == 'confirmed' and confirmation['payment_mode'] == 'mock'
tickets = request('GET', '/api/v1/me/tickets', token=buyer)['items']
assert len(tickets) == 2, 'Repeated checkout must not issue extra tickets'
detail = request('GET', f'/api/v1/tickets/{tickets[0]["id"]}', token=buyer)
qr = request('GET', f'/api/v1/tickets/{detail["id"]}/qr', token=buyer, raw=True)
assert qr.startswith(b'\x89PNG\r\n\x1a\n')
check_in = f'/api/v1/controller/sessions/{session}/check-ins'
request('POST', check_in, {'token': detail['qr_payload']}, token=organizer)
request('POST', check_in, {'token': detail['qr_payload']}, token=organizer, status=409)
print('OK: JWT login, booking, repeated checkout, two tickets, QR, single-use check-in')


def chunk(kind, data):
    return struct.pack('!I', len(data)) + kind + data + struct.pack('!I', zlib.crc32(kind+data) & 0xffffffff)


poster = b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('!2I5B', 1, 1, 8, 6, 0, 0, 0)) + chunk(b'IDAT', zlib.compress(b'\x00\xff\x00\x00\xff')) + chunk(b'IEND', b'')
url = request('POST', f'/api/v1/organizer/events/{event}/poster', poster, token=organizer, organizer=True)['url']
assert request('GET', url, raw=True) == poster
print('OK: private S3 upload and public poster through API')
for attempt in range(30):
    report = request('GET', report_path, token=organizer, organizer=True)
    if report['confirmed']['bookings'] >= before+1:
        assert report['confirmed']['tickets'] >= 2
        print('OK: booking.confirmed delivered through outbox/Kafka to ClickHouse report')
        break
    time.sleep(1)
else:
    raise RuntimeError('Analytics did not catch up: ensure the booking-analytics consumer is running')
