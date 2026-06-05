-- This migration is for databases that still hold legacy decimal money values
-- in major units, for example 1000.5000 meaning NGN 1000.50.
--
-- It multiplies those values by 100 and converts the columns to bigint minor
-- units so the current Go code can treat them as kobo.
--
-- Do not run this blindly if the database has already accepted non-zero writes
-- from the newer kobo-based code while the column types were still decimal,
-- because those rows may already be stored as minor units in a decimal column.
-- In that mixed-data case, audit and clean the data first.

BEGIN;

ALTER TABLE IF EXISTS wallets
	ALTER COLUMN balance TYPE bigint USING ROUND(balance * 100);

ALTER TABLE IF EXISTS transactions
	ALTER COLUMN amount TYPE bigint USING ROUND(amount * 100);

ALTER TABLE IF EXISTS ledger_entries
	ALTER COLUMN debit TYPE bigint USING ROUND(debit * 100),
	ALTER COLUMN credit TYPE bigint USING ROUND(credit * 100),
	ALTER COLUMN balance_after TYPE bigint USING ROUND(balance_after * 100);

ALTER TABLE IF EXISTS transaction_limits
	ALTER COLUMN max_transaction_amount TYPE bigint USING ROUND(max_transaction_amount * 100),
	ALTER COLUMN daily_limit TYPE bigint USING ROUND(daily_limit * 100),
	ALTER COLUMN monthly_limit TYPE bigint USING ROUND(monthly_limit * 100);

ALTER TABLE IF EXISTS transaction_usage
	ALTER COLUMN amount TYPE bigint USING ROUND(amount * 100);

ALTER TABLE IF EXISTS provider_transactions
	ALTER COLUMN amount TYPE bigint USING ROUND(amount * 100),
	ALTER COLUMN fee TYPE bigint USING ROUND(fee * 100);

ALTER TABLE IF EXISTS escrow_orders
	ALTER COLUMN amount TYPE bigint USING ROUND(amount * 100);

COMMIT;
