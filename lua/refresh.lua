-- KEYS = [LOCK_KEY]
-- ARGV = [LOCK_ID, TTL]
local t = redis.call('TYPE', KEYS[1])["ok"]
if (t == "string" and redis.call('GET', KEYS[1]) ~= ARGV[1]) or
        (t == "set" and redis.call('SISMEMBER', KEYS[1], ARGV[1]) == 0) or
        (t == "none") then
    return 0
end

return redis.call('PEXPIRE', KEYS[1], ARGV[2])