-- Development-only, deterministic Shamgarh fixtures. Names, people,
-- addresses, emails, and phone values are fictional. Safe to run repeatedly.
BEGIN;

INSERT INTO users (user_id, phone, name, email, phone_verified, email_verified, role, onboarding_complete)
VALUES
 ('00000000-0000-4000-8000-000000000001', '9000000001', 'Swaad Demo Owner 1', 'demo-owner-1@invalid.swaad.test', TRUE, TRUE, 'restaurant_owner', TRUE),
 ('00000000-0000-4000-8000-000000000002', '9000000002', 'Swaad Demo Owner 2', 'demo-owner-2@invalid.swaad.test', TRUE, TRUE, 'restaurant_owner', TRUE),
 ('00000000-0000-4000-8000-000000000003', '9000000003', 'Swaad Demo Owner 3', 'demo-owner-3@invalid.swaad.test', TRUE, TRUE, 'restaurant_owner', TRUE),
 ('00000000-0000-4000-8000-000000000004', '9000000004', 'Swaad Demo Owner 4', 'demo-owner-4@invalid.swaad.test', TRUE, TRUE, 'restaurant_owner', TRUE),
 ('00000000-0000-4000-8000-000000000005', '9000000005', 'Swaad Demo Owner 5', 'demo-owner-5@invalid.swaad.test', TRUE, TRUE, 'restaurant_owner', TRUE),
 ('00000000-0000-4000-8000-000000000011', '9000000011', 'Swaad Demo Driver 1', 'demo-driver-1@invalid.swaad.test', TRUE, TRUE, 'driver', TRUE),
 ('00000000-0000-4000-8000-000000000012', '9000000012', 'Swaad Demo Driver 2', 'demo-driver-2@invalid.swaad.test', TRUE, TRUE, 'driver', TRUE),
 ('00000000-0000-4000-8000-000000000021', '9000000021', 'Swaad Demo Operations', 'demo-operations@invalid.swaad.test', TRUE, TRUE, 'restaurant_manager', TRUE)
ON CONFLICT (user_id) DO UPDATE SET name = EXCLUDED.name, account_status = 'active', is_deleted = FALSE;

-- Repeatable notification fixtures for the operations and driver demo views.
INSERT INTO notifications (notification_id, recipient_id, recipient_type, channels, title, body, status, is_read)
SELECT '60000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000021', 'restaurant_manager', ARRAY['in_app'], 'Operations workspace ready', 'Mock delivery monitoring is online for Shamgarh.', 'delivered', FALSE
WHERE NOT EXISTS (SELECT 1 FROM notifications WHERE notification_id = '60000000-0000-4000-8000-000000000001');
INSERT INTO notifications (notification_id, recipient_id, recipient_type, channels, title, body, status, is_read)
SELECT '60000000-0000-4000-8000-000000000011', '00000000-0000-4000-8000-000000000011', 'driver', ARRAY['in_app'], 'Demo shift available', 'Set your availability to receive a fictional Shamgarh assignment.', 'delivered', FALSE
WHERE NOT EXISTS (SELECT 1 FROM notifications WHERE notification_id = '60000000-0000-4000-8000-000000000011');
INSERT INTO notifications (notification_id, recipient_id, recipient_type, channels, title, body, status, is_read)
SELECT '60000000-0000-4000-8000-000000000012', '00000000-0000-4000-8000-000000000012', 'driver', ARRAY['in_app'], 'Mock provider active', 'Delivery progression is simulated and restart-safe for this demo.', 'delivered', FALSE
WHERE NOT EXISTS (SELECT 1 FROM notifications WHERE notification_id = '60000000-0000-4000-8000-000000000012');

-- Fictional development-only delivery partners. The encrypted bytea values are
-- placeholders and must never be used as real identity or vehicle documents.
INSERT INTO driver_profiles (user_id, driving_license_number_encrypted, vehicle_registration_encrypted, is_available, current_city)
VALUES
 ('00000000-0000-4000-8000-000000000011', decode('ZGVtby1saWNlbnNlLTE=', 'base64'), decode('ZGVtby12ZWhpY2xlLTE=', 'base64'), TRUE, 'Shamgarh'),
 ('00000000-0000-4000-8000-000000000012', decode('ZGVtby1saWNlbnNlLTI=', 'base64'), decode('ZGVtby12ZWhpY2xlLTI=', 'base64'), TRUE, 'Shamgarh')
ON CONFLICT (user_id) DO UPDATE SET is_available = EXCLUDED.is_available, current_city = EXCLUDED.current_city;

INSERT INTO restaurants (restaurant_id, owner_id, name, description, cuisine_types, address_line1, area, city, state, pincode, latitude, longitude, location, service_radius_km, delivery_time_min, opening_cron, is_open, rating, total_ratings, status)
VALUES
 ('10000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000001', 'Kesar Thali Ghar', 'Comforting vegetarian thalis and seasonal specials.', ARRAY['North Indian','Vegetarian'], 'Main Market Road', 'Shamgarh', 'Shamgarh', 'Madhya Pradesh', '458883', 24.1874, 75.6396, ST_SetSRID(ST_MakePoint(75.6396,24.1874),4326), 8, 35, '0 10 * * *', TRUE, 4.7, 128, 'active'),
 ('10000000-0000-4000-8000-000000000002', '00000000-0000-4000-8000-000000000002', 'Malwa Tadka', 'Malwa-inspired homestyle plates with Jain-friendly choices.', ARRAY['Malwa','Jain Friendly','Vegetarian'], 'Station Link Road', 'Shamgarh', 'Shamgarh', 'Madhya Pradesh', '458883', 24.1849, 75.6462, ST_SetSRID(ST_MakePoint(75.6462,24.1849),4326), 7, 30, '0 9 * * *', TRUE, 4.6, 94, 'active'),
 ('10000000-0000-4000-8000-000000000003', '00000000-0000-4000-8000-000000000003', 'Narmada Bowl Co.', 'Fresh grain bowls, chaas, and light vegetarian meals.', ARRAY['Healthy','Bowls','Vegetarian'], 'Lake View Lane', 'Shamgarh', 'Shamgarh', 'Madhya Pradesh', '458883', 24.1911, 75.6348, ST_SetSRID(ST_MakePoint(75.6348,24.1911),4326), 6, 25, '0 11 * * *', TRUE, 4.5, 76, 'active'),
 ('10000000-0000-4000-8000-000000000004', '00000000-0000-4000-8000-000000000004', 'Chulha Junction', 'Fictional family kitchen serving smoky tandoor favourites.', ARRAY['North Indian','Tandoor'], 'Neem Chowk', 'Shamgarh', 'Shamgarh', 'Madhya Pradesh', '458883', 24.1818, 75.6327, ST_SetSRID(ST_MakePoint(75.6327,24.1818),4326), 9, 40, '0 12 * * *', TRUE, 4.4, 61, 'active'),
 ('10000000-0000-4000-8000-000000000005', '00000000-0000-4000-8000-000000000005', 'Sajilo Snacks', 'Quick fictional snacks, poha, and evening chai.', ARRAY['Snacks','Breakfast','Vegetarian'], 'Bus Stand Road', 'Shamgarh', 'Shamgarh', 'Madhya Pradesh', '458883', 24.1890, 75.6501, ST_SetSRID(ST_MakePoint(75.6501,24.1890),4326), 7, 20, '0 7 * * *', TRUE, 4.3, 48, 'active')
ON CONFLICT (restaurant_id) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description, cuisine_types=EXCLUDED.cuisine_types, address_line1=EXCLUDED.address_line1, area=EXCLUDED.area, city=EXCLUDED.city, state=EXCLUDED.state, pincode=EXCLUDED.pincode, latitude=EXCLUDED.latitude, longitude=EXCLUDED.longitude, location=EXCLUDED.location, service_radius_km=EXCLUDED.service_radius_km, delivery_time_min=EXCLUDED.delivery_time_min, opening_cron=EXCLUDED.opening_cron, is_open=EXCLUDED.is_open, rating=EXCLUDED.rating, total_ratings=EXCLUDED.total_ratings, status=EXCLUDED.status;

INSERT INTO menus (menu_id, restaurant_id) VALUES
 ('20000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001'),
 ('20000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000002'),
 ('20000000-0000-4000-8000-000000000003','10000000-0000-4000-8000-000000000003'),
 ('20000000-0000-4000-8000-000000000004','10000000-0000-4000-8000-000000000004'),
 ('20000000-0000-4000-8000-000000000005','10000000-0000-4000-8000-000000000005')
ON CONFLICT (restaurant_id) DO NOTHING;

INSERT INTO menu_categories (category_id, menu_id, name, sort_order) VALUES
 ('30000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001','Thalis',1),
 ('30000000-0000-4000-8000-000000000002','20000000-0000-4000-8000-000000000002','Malwa Favourites',1),
 ('30000000-0000-4000-8000-000000000003','20000000-0000-4000-8000-000000000003','Signature Bowls',1),
 ('30000000-0000-4000-8000-000000000004','20000000-0000-4000-8000-000000000004','Tandoor',1),
 ('30000000-0000-4000-8000-000000000005','20000000-0000-4000-8000-000000000005','Quick Bites',1)
ON CONFLICT (category_id) DO UPDATE SET name=EXCLUDED.name, sort_order=EXCLUDED.sort_order;

INSERT INTO menu_items (item_id, restaurant_id, category_id, name, description, price, is_veg, is_available, preparation_time_min, tags, sort_order)
VALUES
 ('40000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001','30000000-0000-4000-8000-000000000001','Kesar Special Thali','Dal, seasonal sabzi, roti, rice, salad, and sweet.',219.00,TRUE,TRUE,20,ARRAY['bestseller','veg'],1),
 ('40000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000001','30000000-0000-4000-8000-000000000001','Jain Thali','No onion, no garlic vegetarian thali.',239.00,TRUE,TRUE,22,ARRAY['jain','veg'],2),
 ('40000000-0000-4000-8000-000000000003','10000000-0000-4000-8000-000000000002','30000000-0000-4000-8000-000000000002','Dal Baati Churma','A fictional Malwa-style comfort plate.',189.00,TRUE,TRUE,25,ARRAY['regional','veg'],1),
 ('40000000-0000-4000-8000-000000000004','10000000-0000-4000-8000-000000000003','30000000-0000-4000-8000-000000000003','Narmada Power Bowl','Millets, paneer, greens, and lemon dressing.',249.00,TRUE,TRUE,18,ARRAY['healthy','veg'],1),
 ('40000000-0000-4000-8000-000000000005','10000000-0000-4000-8000-000000000004','30000000-0000-4000-8000-000000000004','Smoky Paneer Tikka','Tandoor-grilled paneer with mint chutney.',229.00,TRUE,TRUE,24,ARRAY['tandoor','veg'],1),
 ('40000000-0000-4000-8000-000000000006','10000000-0000-4000-8000-000000000005','30000000-0000-4000-8000-000000000005','Poha with Sev','Warm poha topped with crunchy sev and coriander.',79.00,TRUE,TRUE,10,ARRAY['breakfast','veg'],1)
ON CONFLICT (item_id) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description, price=EXCLUDED.price, is_veg=EXCLUDED.is_veg, is_available=EXCLUDED.is_available, preparation_time_min=EXCLUDED.preparation_time_min, tags=EXCLUDED.tags, sort_order=EXCLUDED.sort_order, is_deleted=FALSE;

COMMIT;
