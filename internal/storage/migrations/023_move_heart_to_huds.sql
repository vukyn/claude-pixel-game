-- 023_move_heart_to_huds.sql
UPDATE animations
   SET path = 'huds/healthbar/heartbeat.png',
       file = 'heartbeat.png'
 WHERE id = 'heart_beat';
