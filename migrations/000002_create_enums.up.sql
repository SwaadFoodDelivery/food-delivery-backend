DO $$ BEGIN
    CREATE TYPE user_role AS ENUM ('client','restaurant_owner','restaurant_manager','driver');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE order_status AS ENUM ('order_created','pending_payment','confirmed','accepted','preparing','ready_for_pickup','picked_up','out_for_delivery','delivered','cancelled','rejected');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE delivery_status AS ENUM ('assigned','en_route_to_restaurant','arrived_at_restaurant','picked_up','out_for_delivery','delivered');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
