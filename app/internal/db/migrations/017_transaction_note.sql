-- Migration 017: free-text note on transactions, for when the imported
-- description isn't enough to identify what a purchase actually was.

alter table transactions add column note text;
