-- Migration code uploads legacy BYTEA values before this file runs. Refuse to
-- drop the columns if storage is not configured or an upload has failed.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM images WHERE content IS NOT NULL) THEN
        RAISE EXCEPTION 'Legacy images must be migrated: configure S3_BUCKET and restart the API';
    END IF;
    IF EXISTS (SELECT 1 FROM images i WHERE COALESCE(to_jsonb(i)->>'image_url', '') NOT IN ('', '/api/images/' || id)) THEN
        RAISE EXCEPTION 'Legacy external image URLs require manual import to S3 before removing image_url';
    END IF;
END $$;
ALTER TABLE images DROP COLUMN IF EXISTS content;
ALTER TABLE images DROP COLUMN IF EXISTS content_type;
ALTER TABLE images DROP COLUMN IF EXISTS image_url;
