-- +goose Up
-- Reference catalog for the Namangan pilot. Market list and bag weights are
-- starting values to be confirmed during the market visit.

INSERT INTO regions (id, slug, name, kind, parent_id) VALUES
    (1,  'namangan-viloyati', 'Namangan viloyati', 'viloyat', NULL),
    (2,  'namangan-shahri',   'Namangan shahri',   'shahar',  1),
    (3,  'chortoq',           'Chortoq',           'tuman',   1),
    (4,  'chust',             'Chust',             'tuman',   1),
    (5,  'kosonsoy',          'Kosonsoy',          'tuman',   1),
    (6,  'mingbuloq',         'Mingbuloq',         'tuman',   1),
    (7,  'namangan-tumani',   'Namangan tumani',   'tuman',   1),
    (8,  'norin',             'Norin',             'tuman',   1),
    (9,  'pop',               'Pop',               'tuman',   1),
    (10, 'toraqorgon',        'To''raqo''rg''on',  'tuman',   1),
    (11, 'uchqorgon',         'Uchqo''rg''on',     'tuman',   1),
    (12, 'uychi',             'Uychi',             'tuman',   1),
    (13, 'yangiqorgon',       'Yangiqo''rg''on',   'tuman',   1);

INSERT INTO markets (id, slug, name, region_id, kind) VALUES
    (1, 'chorsu-namangan',   'Chorsu dehqon bozori',       2,  'dehqon'),
    (2, 'namangan-ulgurji',  'Namangan ulgurji bozori',    2,  'ulgurji'),
    (3, 'chust-dehqon',      'Chust dehqon bozori',        4,  'dehqon'),
    (4, 'chortoq-dehqon',    'Chortoq dehqon bozori',      3,  'dehqon'),
    (5, 'kosonsoy-dehqon',   'Kosonsoy dehqon bozori',     5,  'dehqon'),
    (6, 'uychi-dehqon',      'Uychi dehqon bozori',        12, 'dehqon'),
    (7, 'toraqorgon-dehqon', 'To''raqo''rg''on dehqon bozori', 10, 'dehqon'),
    (8, 'namangan-qurilish', 'Namangan qurilish bozori',   2,  'qurilish');

INSERT INTO products (id, slug, name, category, base_unit, synonyms) VALUES
    (1,  'kartoshka',   'Kartoshka',       'sabzavot',   'kg',   '{kartoshka,kartofel,kartop,картошка,картофель}'),
    (2,  'piyoz',       'Piyoz',           'sabzavot',   'kg',   '{piyoz,luk,лук,пиёз}'),
    (3,  'sabzi',       'Sabzi',           'sabzavot',   'kg',   '{sabzi,sabzi sariq,морковь,сабзи}'),
    (4,  'pomidor',     'Pomidor',         'sabzavot',   'kg',   '{pomidor,pamidor,tomat,помидор,помидоры}'),
    (5,  'bodring',     'Bodring',         'sabzavot',   'kg',   '{bodring,ogurets,огурец,огурцы,бодринг}'),
    (6,  'karam',       'Karam',           'sabzavot',   'kg',   '{karam,kapusta,капуста,карам}'),
    (7,  'sarimsoq',    'Sarimsoq',        'sabzavot',   'kg',   '{sarimsoq,chesnok,чеснок}'),
    (8,  'bulgor',      'Bulg''or qalampiri', 'sabzavot', 'kg',   '{bulgor,bolgarka,qalampir,перец болгарский}'),
    (9,  'olma',        'Olma',            'meva',       'kg',   '{olma,yabloko,яблоко,яблоки}'),
    (10, 'uzum',        'Uzum',            'meva',       'kg',   '{uzum,vinograd,виноград}'),
    (11, 'anor',        'Anor',            'meva',       'kg',   '{anor,granat,гранат}'),
    (12, 'limon',       'Limon',           'meva',       'kg',   '{limon,лимон}'),
    (13, 'mol-goshti',  'Mol go''shti',    'gosht',      'kg',   '{mol goshti,mol go''shti,govyadina,говядина}'),
    (14, 'qoy-goshti',  'Qo''y go''shti',  'gosht',      'kg',   '{qoy goshti,qo''y go''shti,baranina,баранина}'),
    (15, 'tovuq',       'Tovuq go''shti',  'gosht',      'kg',   '{tovuq,kuritsa,курица,бройлер}'),
    (16, 'tuxum',       'Tuxum',           'oziq_ovqat', 'dona', '{tuxum,yaytso,яйцо,яйца}'),
    (17, 'guruch',      'Guruch',          'don',        'kg',   '{guruch,ris,рис,devzira,лазер}'),
    (18, 'un',          'Un',              'don',        'kg',   '{un,muka,мука}'),
    (19, 'bugdoy',      'Bug''doy',        'don',        'kg',   '{bugdoy,bug''doy,pshenitsa,пшеница}'),
    (20, 'nohat',       'No''xat',         'don',        'kg',   '{noxat,no''xat,nut,нут}'),
    (21, 'mosh',        'Mosh',            'don',        'kg',   '{mosh,mash,маш}'),
    (22, 'paxta-yogi',  'Paxta yog''i',    'oziq_ovqat', 'litr', '{paxta yogi,paxta yog''i,хлопковое масло,yog}'),
    (23, 'kungaboqar-yogi', 'Kungaboqar yog''i', 'oziq_ovqat', 'litr', '{kungaboqar yogi,подсолнечное масло}'),
    (24, 'shakar',      'Shakar',          'oziq_ovqat', 'kg',   '{shakar,sahar,сахар}'),
    (25, 'tuz',         'Tuz',             'oziq_ovqat', 'kg',   '{tuz,sol,соль}'),
    (26, 'sut',         'Sut',             'oziq_ovqat', 'litr', '{sut,moloko,молоко}'),
    (27, 'sement',      'Sement',          'qurilish',   'kg',   '{sement,tsement,цемент}'),
    (28, 'gisht',       'G''isht',         'qurilish',   'dona', '{gisht,g''isht,kirpich,кирпич}'),
    (29, 'armatura',    'Armatura',        'qurilish',   'kg',   '{armatura,арматура}'),
    (30, 'yuk-tashish', 'Yuk tashish',     'xizmat',     'xizmat', '{yuk tashish,gruzoperevozka,грузоперевозка,damas}');

INSERT INTO unit_conversions (product_id, unit, base_quantity) VALUES
    (1,  'qop',    50),
    (2,  'qop',    40),
    (3,  'qop',    40),
    (4,  'yashik', 20),
    (5,  'yashik', 15),
    (9,  'yashik', 18),
    (10, 'yashik', 10),
    (17, 'qop',    50),
    (18, 'qop',    50),
    (24, 'qop',    50),
    (27, 'qop',    50),
    (16, 'karobka', 360),
    (16, 'lotok',   30);

-- +goose Down
DELETE FROM unit_conversions;
DELETE FROM products;
DELETE FROM markets;
DELETE FROM regions;
