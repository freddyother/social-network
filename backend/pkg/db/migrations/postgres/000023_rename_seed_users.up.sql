WITH user_updates (
    id,
    email,
    first_name,
    last_name,
    nickname,
    about_me
) AS (
    VALUES
        ('demo-user-01', 'luna.ortega@example.test', 'Luna', 'Ortega', 'luna_ortega', 'Urban photographer, coffee hunter, and weekend sketchbook keeper.'),
        ('demo-user-02', 'mateo.silva@example.test', 'Mateo', 'Silva', 'mateosilva', 'Frontend tinkerer, cycling commuter, and fan of tidy playlists.'),
        ('demo-user-03', 'sofia.moreau@example.test', 'Sofia', 'Moreau', 'sofia_moreau', 'Book club regular sharing notes on fiction, food, and small discoveries.'),
        ('demo-user-04', 'noah.bennett@example.test', 'Noah', 'Bennett', 'noahbennett', 'Trail walker, amateur cook, and slow Sunday planner.'),
        ('demo-user-05', 'isabel.costa@example.test', 'Isabel', 'Costa', 'isabelcosta', 'Product designer collecting useful patterns and thoughtful details.'),
        ('demo-user-06', 'elias.hart@example.test', 'Elias', 'Hart', 'eliashart', 'Backend learner, puzzle solver, and occasional film reviewer.'),
        ('demo-user-07', 'maya.singh@example.test', 'Maya', 'Singh', 'maya_singh', 'Language student, recipe tester, and local cafe explorer.'),
        ('demo-user-08', 'leo.navarro@example.test', 'Leo', 'Navarro', 'leonavarro', 'Music fan sharing gig notes, guitar practice, and new albums.'),
        ('demo-user-09', 'amara.wilson@example.test', 'Amara', 'Wilson', 'amara_wilson', 'Fitness beginner tracking routines, walks, and tiny wins.'),
        ('demo-user-10', 'theo.martin@example.test', 'Theo', 'Martin', 'theomartin', 'Indie film watcher, photo walk organizer, and tea loyalist.'),
        ('demo-user-11', 'valeria.rossi@example.test', 'Valeria', 'Rossi', 'valeria_rossi', 'Home cook trading dinner ideas and market finds.'),
        ('demo-user-12', 'nico.alvarez@example.test', 'Nico', 'Alvarez', 'nicoalvarez', 'Small-project builder documenting progress and lessons learned.'),
        ('demo-user-13', 'eva.kim@example.test', 'Eva', 'Kim', 'evakim', 'Ceramics hobbyist, plant parent, and weekend museum wanderer.'),
        ('demo-user-14', 'julian.price@example.test', 'Julian', 'Price', 'julianprice', 'Runner, map collector, and breakfast enthusiast.'),
        ('demo-user-15', 'camila.lopez@example.test', 'Camila', 'Lopez', 'camila_lopez', 'Community organizer sharing group updates and city finds.'),
        ('demo-user-16', 'lucas.meyer@example.test', 'Lucas', 'Meyer', 'lucasmeyer', 'Developer, chess learner, and notebook addict.'),
        ('demo-user-17', 'sara.dubois@example.test', 'Sara', 'Dubois', 'saradubois', 'Travel planner, gallery browser, and postcard sender.'),
        ('demo-user-18', 'daniel.brooks@example.test', 'Daniel', 'Brooks', 'danielbrooks', 'Home barista testing beans, tools, and quiet workflows.'),
        ('demo-user-19', 'nina.petrov@example.test', 'Nina', 'Petrov', 'ninapetrov', 'Illustrator sharing drafts, references, and creative routines.'),
        ('demo-user-20', 'oscar.reed@example.test', 'Oscar', 'Reed', 'oscarreed', 'Gardener, podcast listener, and weekend project finisher.')
)
UPDATE users AS u
SET
    email = user_updates.email,
    first_name = user_updates.first_name,
    last_name = user_updates.last_name,
    nickname = user_updates.nickname,
    about_me = user_updates.about_me,
    updated_at = NOW()
FROM user_updates
WHERE u.id = user_updates.id;

WITH post_updates (
    id,
    title,
    body
) AS (
    VALUES
        ('demo-post-01', 'Morning light downtown', 'Found a quiet corner before work and saved the view for later.'),
        ('demo-post-02', 'A cleaner project board', 'Reworked my task list today and everything feels easier to scan.'),
        ('demo-post-03', 'Book notes and coffee', 'The latest chapter gave our reading group plenty to argue about.'),
        ('demo-post-04', 'Trail weather held up', 'The clouds looked dramatic, but the path stayed dry all afternoon.'),
        ('demo-post-05', 'Interface details', 'Collecting small UI ideas that make repeated workflows feel calmer.'),
        ('demo-post-06', 'Learning log', 'Finally untangled the bug that was hiding in my assumptions.'),
        ('demo-post-07', 'Lunch experiment', 'Trying a new recipe and keeping notes for the next version.'),
        ('demo-post-08', 'New playlist energy', 'A few tracks carried the whole evening better than expected.'),
        ('demo-post-09', 'Small training win', 'Nothing heroic, just consistency showing up in tiny increments.'),
        ('demo-post-10', 'Film night pick', 'Tonight calls for subtitles, snacks, and no second-screen scrolling.'),
        ('demo-post-11', 'Market haul', 'Fresh herbs changed the entire plan for dinner.'),
        ('demo-post-12', 'Build notes', 'A small feature landed today and the rough edges are finally softer.'),
        ('demo-post-13', 'Studio shelf reset', 'Moved a few tools around and the workspace immediately felt lighter.'),
        ('demo-post-14', 'Long route home', 'Took the scenic way back and found a new breakfast place.'),
        ('demo-post-15', 'Neighborhood noticeboard', 'A few local events are worth saving for the weekend.'),
        ('demo-post-16', 'Chess puzzle streak', 'The answer was obvious only after ten very stubborn minutes.'),
        ('demo-post-17', 'Gallery afternoon', 'One room, three paintings, and a lot of notes.'),
        ('demo-post-18', 'Coffee dial-in', 'Changed the grind and the cup finally opened up.'),
        ('demo-post-19', 'Sketchbook page', 'Trying looser lines and letting the first draft breathe.'),
        ('demo-post-20', 'Garden progress', 'The planters are settling in and the balcony looks alive again.')
)
UPDATE posts AS p
SET
    title = post_updates.title,
    body = post_updates.body,
    updated_at = NOW()
FROM post_updates
WHERE p.id = post_updates.id;

WITH comment_updates (
    id,
    body
) AS (
    VALUES
        ('demo-comment-01', 'Love this update. The photo makes the whole post feel alive.'),
        ('demo-comment-02', 'This sounds useful. I might borrow the idea for my own week.'),
        ('demo-comment-03', 'Nice one. The details here are exactly what I wanted to see.'),
        ('demo-comment-04', 'Adding this to my list for later.'),
        ('demo-comment-05', 'That looks like a properly good afternoon.'),
        ('demo-comment-06', 'The caption sold me before I even noticed the image.'),
        ('demo-comment-07', 'I want to try something similar this weekend.'),
        ('demo-comment-08', 'Great timing. This is the kind of post I needed today.'),
        ('demo-comment-09', 'Small wins count. This one is worth keeping.'),
        ('demo-comment-10', 'I am here for this energy.'),
        ('demo-comment-11', 'That sounds excellent. Please keep the updates coming.'),
        ('demo-comment-12', 'Clean, simple, and useful. Nicely done.'),
        ('demo-comment-13', 'This has such a good mood.'),
        ('demo-comment-14', 'Saved for future inspiration.'),
        ('demo-comment-15', 'That is a strong weekend plan.'),
        ('demo-comment-16', 'I respect the stubborn ten minutes.'),
        ('demo-comment-17', 'Beautiful note. The image adds a lot.'),
        ('demo-comment-18', 'Now I want coffee. This is persuasive.'),
        ('demo-comment-19', 'The looser style works really well.'),
        ('demo-comment-20', 'Balcony progress is still progress.')
)
UPDATE comments AS c
SET
    body = comment_updates.body,
    updated_at = NOW()
FROM comment_updates
WHERE c.id = comment_updates.id;

WITH group_post_updates (
    id,
    body
) AS (
    VALUES
        ('demo-group-post-01', 'Sharing a new project photo and a few notes from this week.'),
        ('demo-group-post-02', 'Route idea for the next walk, with a couple of reference shots.'),
        ('demo-group-post-03', 'This week''s reading prompt, plus images for the mood board.'),
        ('demo-group-post-04', 'Kitchen experiment update with photos from the process.'),
        ('demo-group-post-05', 'Film night thread: poster inspiration and watch notes.'),
        ('demo-group-post-06', 'Progress update from the coding desk, screenshots included.'),
        ('demo-group-post-07', 'Photo walk highlights from the latest route.'),
        ('demo-group-post-08', 'Training check-in with a visual recap.'),
        ('demo-group-post-09', 'Conversation prompt for the cafe meetup.'),
        ('demo-group-post-10', 'Playlist exchange thread with cover inspiration.')
)
UPDATE group_posts AS gp
SET
    body = group_post_updates.body,
    updated_at = NOW()
FROM group_post_updates
WHERE gp.id = group_post_updates.id;
