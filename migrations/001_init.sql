-- Concert ticketing schema (BookMyShow-proof design)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE sale_phase AS ENUM (
  'pre_register',
  'lottery_draw',
  'presale',
  'general_sale',
  'ended'
);

CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  premium_member BOOLEAN NOT NULL DEFAULT FALSE,
  partner_card BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  venue TEXT NOT NULL,
  starts_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE sales (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id UUID NOT NULL REFERENCES events(id),
  phase sale_phase NOT NULL DEFAULT 'pre_register',
  opens_at TIMESTAMPTZ NOT NULL,
  ends_at TIMESTAMPTZ NOT NULL,
  max_tickets_per_user INT NOT NULL DEFAULT 4,
  waiting_room_cap INT NOT NULL DEFAULT 1000000,
  admit_per_minute INT NOT NULL DEFAULT 10000,
  total_seats INT NOT NULL DEFAULT 20000,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE seats (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sale_id UUID NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
  section TEXT NOT NULL,
  row_num INT NOT NULL,
  seat_num INT NOT NULL,
  status TEXT NOT NULL DEFAULT 'available' CHECK (status IN ('available', 'held', 'sold')),
  version INT NOT NULL DEFAULT 0,
  UNIQUE (sale_id, section, row_num, seat_num)
);

CREATE TABLE queue_entries (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sale_id UUID NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id),
  position BIGINT NOT NULL,
  joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  admitted_at TIMESTAMPTZ,
  UNIQUE (sale_id, user_id),
  UNIQUE (sale_id, position)
);

CREATE INDEX idx_queue_entries_sale_position ON queue_entries (sale_id, position);

CREATE TABLE pre_registrations (
  sale_id UUID NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id),
  registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (sale_id, user_id)
);

CREATE TABLE access_codes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sale_id UUID NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id),
  code_hash TEXT NOT NULL UNIQUE,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE holds (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sale_id UUID NOT NULL REFERENCES sales(id),
  user_id UUID NOT NULL REFERENCES users(id),
  seat_ids JSONB NOT NULL,
  idempotency_key TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (sale_id, user_id, idempotency_key)
);

CREATE TABLE orders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sale_id UUID NOT NULL REFERENCES sales(id),
  user_id UUID NOT NULL REFERENCES users(id),
  hold_id UUID REFERENCES holds(id),
  idempotency_key TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'failed')),
  total_cents INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (sale_id, user_id, idempotency_key)
);

CREATE TABLE tickets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id UUID NOT NULL REFERENCES orders(id),
  sale_id UUID NOT NULL REFERENCES sales(id),
  user_id UUID NOT NULL REFERENCES users(id),
  seat_id UUID NOT NULL REFERENCES seats(id),
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'cancelled')),
  owner_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (seat_id)
);

CREATE INDEX idx_tickets_sale_user ON tickets (sale_id, user_id);
