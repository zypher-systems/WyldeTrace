DROP TRIGGER IF EXISTS trg_ip_in_subnet ON ip_addresses;
DROP FUNCTION IF EXISTS check_ip_in_subnet();
DROP TABLE IF EXISTS ip_addresses;
DROP TABLE IF EXISTS subnets;
DROP TABLE IF EXISTS device_types;
DROP TABLE IF EXISTS locations;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS ip_status;
DROP TYPE IF EXISTS user_role;
