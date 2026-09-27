UPDATE user_settings
SET activity_visibility = 'instance'
WHERE activity_visibility = 'private';
