SET search_path TO racing;

\echo 'Getting test fixtures...'

\echo 'HORSE_ID:'
SELECT horse_id FROM runners GROUP BY horse_id ORDER BY COUNT(*) DESC LIMIT 1;

\echo 'TRAINER_ID:'
SELECT trainer_id FROM runners WHERE trainer_id IS NOT NULL GROUP BY trainer_id ORDER BY COUNT(*) DESC LIMIT 1;

\echo 'JOCKEY_ID:'
SELECT jockey_id FROM runners WHERE jockey_id IS NOT NULL GROUP BY jockey_id ORDER BY COUNT(*) DESC LIMIT 1;

\echo 'COURSE_ID:'
SELECT course_id FROM courses WHERE region IN ('GB','IRE') ORDER BY course_name LIMIT 1;

\echo 'RACE_ID:'
SELECT race_id FROM races WHERE race_date >= current_date - interval '365 days' AND ran >= 10 ORDER BY race_date DESC LIMIT 1;

\echo 'DATE1:'
SELECT race_date FROM races WHERE race_date >= '2024-01-01' GROUP BY race_date ORDER BY COUNT(*) DESC LIMIT 1;

\echo 'DATE2:'
SELECT race_date + 1 FROM races WHERE race_date >= '2024-01-01' GROUP BY race_date ORDER BY COUNT(*) DESC LIMIT 1;

