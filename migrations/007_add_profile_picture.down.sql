-- Remove profile_picture column from users table
ALTER TABLE users DROP COLUMN IF EXISTS profile_picture;
