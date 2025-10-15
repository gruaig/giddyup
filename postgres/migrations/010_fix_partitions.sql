-- Migration: Add prelim column to parent races table
-- Date: 2025-10-15
-- Note: Cannot add columns directly to partitions, must add to parent table

-- The column was already added to the parent table in migration 009
-- Just verify it cascaded to partitions

SELECT 
    t.tablename,
    CASE WHEN c.column_name IS NOT NULL THEN 'YES' ELSE 'NO' END as has_prelim
FROM pg_tables t
LEFT JOIN information_schema.columns c 
    ON c.table_schema = t.schemaname 
    AND c.table_name = t.tablename 
    AND c.column_name = 'prelim'
WHERE t.schemaname = 'racing' 
    AND (t.tablename = 'races' OR t.tablename LIKE 'races_2025_%')
ORDER BY t.tablename;

