-- 0039_vip 回滚：删会员四表 + 移除 pay_order.sku 列。
-- 顺序与 up 相反：先删依赖 pay_order.sku 的 vip_order，再删其余表，最后删列。
DROP TABLE IF EXISTS `vip_coin_grant`;
DROP TABLE IF EXISTS `vip_subscription`;
DROP TABLE IF EXISTS `vip_order`;
DROP TABLE IF EXISTS `membership`;
DROP TABLE IF EXISTS `vip_plan`;

ALTER TABLE `pay_order` DROP COLUMN `sku`;
