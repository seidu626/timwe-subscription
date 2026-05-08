SELECT * FROM public.invalid_msisdn_logs
ORDER BY id ASC LIMIT 100;

select * from public.subscriptions
--where user_identifier = '233272605765'
ORDER BY id DESC LIMIT 5;

select count(*) from public.subscriptions;

select * from public.products;

SELECT COUNT(*) FROM public.invalid_msisdn_logs;

select * from public.notifications 
where type != 'CHARGE' and type != 'USER_RENEWED' and type != 'USER_OPTOUT' and type != 'RENEWAL'
limit 10;

SELECT * FROM public.resubscription_tracking
ORDER BY id ASC LIMIT 100

select * from public.notifications 
where "type" = 'USER_OPTOUT' and  created_at >= '2025-01-01'

select * from public.notifications 
where ("type" = 'USER_RENEWED' or "type" = 'CHARGE')
and created_at >= '2025-08-25' and created_at < '2025-08-26'

ORDER BY id DESC LIMIT 5;


