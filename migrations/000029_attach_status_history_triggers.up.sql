-- Migration 16 defined these functions without attaching triggers. Record
-- future transitions transactionally; do not invent historical events.
CREATE OR REPLACE FUNCTION trigger_emit_order_status_history() RETURNS TRIGGER AS $$
BEGIN
  IF NEW.status IS DISTINCT FROM OLD.status THEN
    INSERT INTO order_status_history(order_id,order_created_at,from_status,to_status,changed_by,changed_at)
    VALUES (NEW.order_id,NEW.created_at,OLD.status,NEW.status,NEW.updated_by,clock_timestamp());
  END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION trigger_emit_delivery_status_history() RETURNS TRIGGER AS $$
BEGIN
  IF NEW.status IS DISTINCT FROM OLD.status THEN
    INSERT INTO delivery_status_history(delivery_id,from_status,to_status,changed_by,changed_at)
    VALUES (NEW.delivery_id,OLD.status,NEW.status,NEW.updated_by,clock_timestamp());
  END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;

CREATE TRIGGER swaad_order_status_history
AFTER UPDATE OF status ON orders
FOR EACH ROW EXECUTE FUNCTION trigger_emit_order_status_history();

CREATE TRIGGER swaad_delivery_status_history
AFTER UPDATE OF status ON deliveries
FOR EACH ROW EXECUTE FUNCTION trigger_emit_delivery_status_history();
