-- PostgreSQL cannot remove enum values safely; leave 'reportview' in place.
-- Reassign any reportview users before downgrading application code:
-- UPDATE users SET role = 'readonly' WHERE role = 'reportview';
SELECT 1;
