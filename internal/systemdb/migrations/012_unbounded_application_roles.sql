-- Пользователю назначается сколько угодно ролей: у прототипа предела нет
-- (справка, ПользовательИнформационнойБазы.Роли), а в erp ролей 1211, больше
-- прежних 1024. Ограничение снимается по его определению, а не по имени:
-- имя ему дал PostgreSQL, и полагаться на то, каким оно вышло, незачем.
-- Остальные проверки списка ролей - без NULL и без нулевого идентификатора -
-- остаются.
DO $$
DECLARE
    bound TEXT;
BEGIN
    FOR bound IN
        SELECT conname FROM pg_constraint
        WHERE conrelid = 'ml_system.application_role_assignments'::regclass
          AND contype = 'c'
          AND pg_get_constraintdef(oid) LIKE '%cardinality(role_ids)%'
    LOOP
        EXECUTE format('ALTER TABLE ml_system.application_role_assignments DROP CONSTRAINT %I', bound);
    END LOOP;
END
$$;
