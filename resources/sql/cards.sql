-- name: count-cards
-- Counts the number of records existent in the cards table
SELECT COUNT(*) FROM cards;

-- name: select-all-cards
-- Selects all cards
SELECT * FROM cards;

-- name: find-card-by-id
-- Find a card by id
SELECT * FROM cards WHERE id = :id LIMIT 1;

-- name: insert-card<!
-- Inserts a new record into the cards table
INSERT INTO cards (title, contents) VALUES (:title, :contents);

-- name: update-card-by-id!
-- Updates the card that has the given id
UPDATE cards SET title = :title, contents = :contents WHERE id = :id;

-- name: delete-card-by-id!
-- Deletes the card that has the given id
DELETE FROM cards WHERE id = :id;
