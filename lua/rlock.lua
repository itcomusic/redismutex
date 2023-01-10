-- KEYS = [LOCK_KEY, LOCK_INTENT]
-- ARGV = [LOCK_ID, TTL]
local t = redis.call('TYPE', KEYS[1])["ok"]
if t == "string" then
   return redis.call('PTTL', KEYS[1])
end

if redis.call("EXISTS", KEYS[2]) == 1 then
   return redis.call('PTTL', KEYS[2])
end

redis.call('SADD', KEYS[1], ARGV[1])
redis.call('PEXPIRE', KEYS[1], ARGV[2])
return nil