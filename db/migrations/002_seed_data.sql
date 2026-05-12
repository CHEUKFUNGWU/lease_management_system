-- +goose Up
-- +goose StatementBegin
INSERT INTO legal_entities (id, code, name, country, currency, is_active)
VALUES 
    ('a1b2c3d4-e5f6-7890-abcd-ef1234567890', 'LE001', '零售集团总公司', 'CN', 'CNY', true),
    ('b2c3d4e5-f6a7-8901-bcde-f12345678901', 'LE002', '零售集团上海公司', 'CN', 'CNY', true)
ON CONFLICT (code) DO NOTHING;

INSERT INTO stores (id, code, name, legal_entity_id, brand, region, is_active)
VALUES 
    ('c3d4e5f6-a7b8-9012-cdef-123456789012', 'ST001', '南京东路旗舰店', 'a1b2c3d4-e5f6-7890-abcd-ef1234567890', '主品牌', '华东', true),
    ('d4e5f6a7-b8c9-0123-def1-234567890123', 'ST002', '淮海路店', 'b2c3d4e5-f6a7-8901-bcde-f12345678901', '主品牌', '华东', true)
ON CONFLICT (code) DO NOTHING;

INSERT INTO landlords (id, code, name, contact_person, contact_email, is_active)
VALUES 
    ('e5f6a7b8-c9d0-1234-ef12-345678901234', 'LL001', '上海商业地产集团', '张先生', 'zhang@property.com', true),
    ('f6a7b8c9-d0e1-2345-f123-456789012345', 'LL002', '北京购物中心管理', '李女士', 'li@shopping.com', true)
ON CONFLICT (code) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM landlords WHERE code IN ('LL001', 'LL002');
DELETE FROM stores WHERE code IN ('ST001', 'ST002');
DELETE FROM legal_entities WHERE code IN ('LE001', 'LE002');
-- +goose StatementEnd
