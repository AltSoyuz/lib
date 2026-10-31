-- Wrong tries on a sign-in code (backoffice.VerifyCode): the code is closed at the 5th.
ALTER TABLE magic_links ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0;
