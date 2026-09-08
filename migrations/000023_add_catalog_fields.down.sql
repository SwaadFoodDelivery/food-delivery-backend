DROP INDEX IF EXISTS idx_menu_categories_menu_sort;
DROP INDEX IF EXISTS idx_menu_items_restaurant_available;
DROP INDEX IF EXISTS idx_restaurants_city;
DROP INDEX IF EXISTS idx_restaurants_status_active;

ALTER TABLE menu_items
    DROP COLUMN IF EXISTS sort_order,
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS preparation_time_min,
    DROP COLUMN IF EXISTS is_veg,
    DROP COLUMN IF EXISTS image_s3_key,
    DROP COLUMN IF EXISTS description;

ALTER TABLE restaurants
    DROP COLUMN IF EXISTS logo_s3_key,
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS total_ratings,
    DROP COLUMN IF EXISTS rating,
    DROP COLUMN IF EXISTS is_open,
    DROP COLUMN IF EXISTS opening_cron,
    DROP COLUMN IF EXISTS delivery_time_min,
    DROP COLUMN IF EXISTS service_radius_km,
    DROP COLUMN IF EXISTS longitude,
    DROP COLUMN IF EXISTS latitude,
    DROP COLUMN IF EXISTS pincode,
    DROP COLUMN IF EXISTS state,
    DROP COLUMN IF EXISTS city,
    DROP COLUMN IF EXISTS area,
    DROP COLUMN IF EXISTS address_line2,
    DROP COLUMN IF EXISTS address_line1;
