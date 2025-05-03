-- check and del
if redis.call('get', KEYS[1]) == ARGV[1] then
    -- 如果是你的锁，调用删除
    -- del返回值是你删除的行数
    return redis.call('del', KEYS[1])
else
    -- 不是你的锁
    return 0
end