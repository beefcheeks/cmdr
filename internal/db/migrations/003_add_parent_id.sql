ALTER TABLE claude_tasks ADD COLUMN parent_id INTEGER REFERENCES claude_tasks(id);
