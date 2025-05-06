val = redis.call('get', KEYS[1])
-- 如果可以、不存在。lua脚本会将val转化为false
if val == false then
    -- key不存在
    -- 这里返回值是 OK
    return redis.call('set', KEYS[1], ARGV[1], 'EX', ARGV[2])
elseif val == ARGV[1] then
    -- 成功获取到val，我们就刷新过期时间
    -- EXPIRE返回值是 数字1或0，所以为了保持一致，我们自己定义
    redis.call('EXPIRE', KEYS[1], ARGV[2])
    return 'OK'
else
    -- 说明锁被人持有
    return ''
end