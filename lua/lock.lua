-- KEYS = [LOCK_KEY, LOCK_INTENT]
-- ARGV = [LOCK_ID, TTL, ENABLE_LOCK_INTENT]
if not redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2], "NX") then
    if ARGV[3] == "1" then
        redis.call("SET", KEYS[2], 1, "PX", ARGV[2])
    end
    return redis.call("PTTL", KEYS[1])
end

redis.call("DEL", KEYS[2])
return nil