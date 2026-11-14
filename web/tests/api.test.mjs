// No npm dependencies: node web/tests/api.test.mjs
import test from 'node:test';
import assert from 'node:assert/strict';

async function client(fetcher) {
  const stored = new Map();
  globalThis.sessionStorage = {getItem:key=>stored.get(key)||null,setItem:(key,value)=>stored.set(key,value),removeItem:key=>stored.delete(key)};
  globalThis.fetch = fetcher;
  return import(`../static/api.js?case=${Math.random()}`);
}
const json = (data,status=200,headers={}) => new Response(JSON.stringify(data),{status,headers:{'Content-Type':'application/json',...headers}});

test('Concurrent private requests share one refresh and use the rotated access token',async()=>{
 let refreshes=0,privateRequests=0;
 const c=await client(async(path,options)=>{
  if(path.endsWith('/auth/refresh')) { refreshes++; await new Promise(resolve=>setTimeout(resolve,5)); return json({access_token:'new-access',refresh_token:'new-refresh'}); }
  privateRequests++; assert.equal(options.headers.Authorization,'Bearer new-access'); return json({ok:true});
 });
 sessionStorage.setItem('ticketrush.refresh','old-refresh');
 await Promise.all([c.api('/me',{auth:true}),c.api('/me/bookings',{auth:true})]);
 assert.equal(refreshes,1); assert.equal(privateRequests,2); assert.equal(sessionStorage.getItem('ticketrush.refresh'),'new-refresh');
});
test('An expired access token refreshes once; public requests carry no bearer',async()=>{
 let calls=0;
 const c=await client(async(path,options)=>{
  if(path.endsWith('/auth/refresh')) return json({access_token:'new',refresh_token:'rotated'});
  if(path.endsWith('/events')) { assert.equal(options.headers.Authorization,undefined); return json([]); }
  calls++; return options.headers.Authorization==='Bearer old' ? json({},401) : json({ok:true});
 });
 c.setSession({access_token:'old',refresh_token:'refresh'});
 assert.deepEqual(await c.api('/me',{auth:true}),{ok:true}); assert.equal(calls,2); await c.api('/events');
});
test('A refresh response cannot restore a session after logout',async()=>{
 let complete;
 const c=await client(()=>new Promise(resolve=>{complete=resolve;}));
 sessionStorage.setItem('ticketrush.refresh','refresh');
 const pending=c.api('/me',{auth:true}); c.clearSession();
 complete(json({access_token:'stale',refresh_token:'stale-refresh'}));
 await assert.rejects(pending,error=>error.status===401); assert.equal(c.hasSession(),false);
});
test('An uncertain POST is never automatically repeated',async()=>{
 let calls=0;
 const c=await client(async()=>{calls++;throw new TypeError('network');});
 c.setSession({access_token:'access',refresh_token:'refresh'});
 await assert.rejects(c.api('/events/id/sessions/id/bookings',{auth:true,method:'POST',data:{quantity:1}}),error=>error.status===0);
 assert.equal(calls,1);
});
test('Rate-limit responses preserve Retry-After for the interface',async()=>{
 const c=await client(async()=>json({code:'rate_limited'},429,{'Retry-After':'17'}));
 await assert.rejects(c.api('/auth/login',{method:'POST',data:{}}),error=>error.status===429&&c.message(error).includes('17'));
});

test('Decimal prices keep kopecks even near the SQL precision limit',async()=>{
 const {money,quantityMoney,itemsMoney}=await import('../static/ui.js');
 assert.match(money('9999999999999999.99'),/9[\s\u00a0\u202f]999[\s\u00a0\u202f]999[\s\u00a0\u202f]999[\s\u00a0\u202f]999[\s\u00a0\u202f]999,99/);
 assert.match(quantityMoney('1500.25',2),/3[\s\u00a0\u202f]000,50/);
 assert.match(itemsMoney([{quantity:3,unit_price:{amount:'0.10'}},{quantity:1,unit_price:{amount:'0.20'}}]),/0,50/);
});
