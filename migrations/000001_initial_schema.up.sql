CREATE TABLE IF NOT EXISTS exchange_items (
    id BIGSERIAL PRIMARY KEY,
    identity_key TEXT NOT NULL,
    item_id INTEGER NOT NULL,
    name VARCHAR(255),
    category_id INTEGER NOT NULL,
    server VARCHAR(100),
    zone VARCHAR(100),
    price BIGINT,
    quantity INTEGER,
    listing_count INTEGER,
    buyer_count INTEGER DEFAULT 0,
    quota BIGINT DEFAULT 0,
    end_time BIGINT,
    sold_out BOOLEAN DEFAULT FALSE,
    in_stock BOOLEAN DEFAULT TRUE,
    last_seen_at TIMESTAMP WITH TIME ZONE,
    modified BOOLEAN DEFAULT FALSE,
    refine_level INTEGER DEFAULT 0,
    is_broken BOOLEAN DEFAULT FALSE,
    buff_id INTEGER,
    buff_attr1_name VARCHAR(100),
    buff_attr1_value INTEGER,
    buff_attr2_name VARCHAR(100),
    buff_attr2_value INTEGER,
    buff_attr3_name VARCHAR(100),
    buff_attr3_value INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_exchange_items_identity_key ON exchange_items(identity_key);
CREATE INDEX IF NOT EXISTS idx_exchange_items_item_id ON exchange_items(item_id);
CREATE INDEX IF NOT EXISTS idx_exchange_items_category_id ON exchange_items(category_id);
CREATE INDEX IF NOT EXISTS idx_exchange_items_server_zone ON exchange_items(server, zone);
CREATE INDEX IF NOT EXISTS idx_exchange_items_in_stock ON exchange_items(in_stock);
CREATE INDEX IF NOT EXISTS idx_exchange_items_last_seen_at ON exchange_items(last_seen_at);

CREATE TABLE IF NOT EXISTS boss_records (
    id BIGSERIAL PRIMARY KEY,
    server VARCHAR(100) NOT NULL,
    zone VARCHAR(100) NOT NULL,
    boss_id INTEGER NOT NULL,
    boss_name VARCHAR(255),
    spawn_time TIMESTAMP WITH TIME ZONE,
    death_time TIMESTAMP WITH TIME ZONE,
    refresh_time TIMESTAMP WITH TIME ZONE,
    is_alive BOOLEAN DEFAULT TRUE,
    killer_name VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_boss_records_server_zone ON boss_records(server, zone);
CREATE INDEX IF NOT EXISTS idx_boss_records_boss_id ON boss_records(boss_id);
CREATE INDEX IF NOT EXISTS idx_boss_records_spawn_time ON boss_records(spawn_time);

CREATE TABLE IF NOT EXISTS pvp_rankings (
    id BIGSERIAL PRIMARY KEY,
    server VARCHAR(100) NOT NULL,
    season INTEGER NOT NULL,
    rank INTEGER NOT NULL,
    character_name VARCHAR(255) NOT NULL,
    team_name VARCHAR(255),
    rating INTEGER,
    wins INTEGER,
    losses INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pvp_rankings_server_season ON pvp_rankings(server, season);
CREATE INDEX IF NOT EXISTS idx_pvp_rankings_rank ON pvp_rankings(rank);
CREATE INDEX IF NOT EXISTS idx_pvp_rankings_created_at ON pvp_rankings(created_at);

CREATE TABLE IF NOT EXISTS auction_items (
    id BIGSERIAL PRIMARY KEY,
    batch_id BIGINT NOT NULL,
    item_id INTEGER NOT NULL,
    item_name VARCHAR(255),
    start_price BIGINT NOT NULL,
    end_price BIGINT,
    seller VARCHAR(255),
    buyer VARCHAR(255),
    result VARCHAR(50),
    bid_count INTEGER DEFAULT 0,
    auction_state VARCHAR(50),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_auction_items_batch_id ON auction_items(batch_id);
CREATE INDEX IF NOT EXISTS idx_auction_items_item_id ON auction_items(item_id);
CREATE INDEX IF NOT EXISTS idx_auction_items_created_at ON auction_items(created_at);

CREATE TABLE IF NOT EXISTS auction_bids (
    id BIGSERIAL PRIMARY KEY,
    batch_id BIGINT NOT NULL,
    item_id INTEGER NOT NULL,
    bidder_name VARCHAR(255),
    bid_amount BIGINT NOT NULL,
    event_type VARCHAR(50),
    bid_time TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_auction_bids_batch_id_item_id ON auction_bids(batch_id, item_id);
CREATE INDEX IF NOT EXISTS idx_auction_bids_bidder_name ON auction_bids(bidder_name);
CREATE INDEX IF NOT EXISTS idx_auction_bids_created_at ON auction_bids(created_at);
