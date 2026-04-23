-- 030_reroot_anim_paths_under_sprites.sql
-- ASSETS_DIR broadened from ./assets/sprites to ./assets so HUD assets
-- (./assets/huds/...) resolve correctly. Existing sprite-based anims need
-- their paths prefixed with "sprites/" to keep loading from the same files.
-- The heart + stamina_bar rows already use "huds/..." paths, so skip them.
UPDATE animations
   SET path = 'sprites/' || path
 WHERE id NOT IN ('heart_beat', 'stamina_bar');
