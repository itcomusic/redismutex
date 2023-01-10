-- KEYS = [LOCK_KEY]
-- ARGV = [LOCK_ID]
local t = redis.call('TYPE', KEYS[1])["ok"]
if t == "string" and redis.call('GET', KEYS[1]) == ARGV[1] then
    return redis.call('DEL', KEYS[1])
elseif t == "set" and redis.call('SISMEMBER', KEYS[1], ARGV[1]) == 1 then
    redis.call('SREM', KEYS[1], ARGV[1])
    if redis.call('SCARD', KEYS[1]) == 0 then
        return redis.call('DEL', KEYS[1])
    end
end
return 1