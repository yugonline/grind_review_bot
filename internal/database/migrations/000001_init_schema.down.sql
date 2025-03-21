-- Drop indices
DROP INDEX IF EXISTS idx_problem_tags_tag_id;
DROP INDEX IF EXISTS idx_problem_tags_problem_id;
DROP INDEX IF EXISTS idx_problems_status_solved_at;
DROP INDEX IF EXISTS idx_problems_category;
DROP INDEX IF EXISTS idx_problems_difficulty;
DROP INDEX IF EXISTS idx_problems_solved_at;
DROP INDEX IF EXISTS idx_problems_status;
DROP INDEX IF EXISTS idx_problems_user_id;

-- Drop tables in reverse order of creation (to respect foreign keys)
DROP TABLE IF EXISTS problem_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS problems;