CREATE OR REPLACE PROCEDURE auth.get_user_count(OUT user_count INTEGER)
LANGUAGE plpgsql
AS $$
BEGIN
    SELECT COUNT(*)::INTEGER
    INTO user_count
    FROM auth.users;
END;
$$;
