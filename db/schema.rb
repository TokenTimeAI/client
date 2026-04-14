# This file is auto-generated from the current state of the database. Instead
# of editing this file, please use the migrations feature of Active Record to
# incrementally modify your database, and then regenerate this schema definition.
#
# This file is the source Rails uses to define your schema when running `bin/rails
# db:schema:load`. When creating a new database, `bin/rails db:schema:load` tends to
# be faster and is potentially less error prone than running all of your
# migrations from scratch. Old migrations may fail to apply correctly if those
# migrations use external dependencies or application code.
#
# It's strongly recommended that you check this file into your version control system.

ActiveRecord::Schema[8.1].define(version: 2026_04_14_001714) do
  # These are extensions that must be enabled in order to support this database
  enable_extension "pg_catalog.plpgsql"

  create_table "api_keys", id: :string, force: :cascade do |t|
    t.boolean "active", default: true, null: false
    t.datetime "created_at", null: false
    t.string "key", null: false
    t.datetime "last_used_at"
    t.string "name", null: false
    t.datetime "updated_at", null: false
    t.string "user_id", null: false
    t.index ["key"], name: "index_api_keys_on_key", unique: true
    t.index ["user_id"], name: "index_api_keys_on_user_id"
  end

  create_table "heartbeat_events", id: :string, force: :cascade do |t|
    t.string "agent_type", null: false
    t.string "branch"
    t.decimal "cost_usd", precision: 10, scale: 8
    t.datetime "created_at", null: false
    t.string "entity"
    t.string "entity_type"
    t.boolean "is_write", default: false
    t.string "language"
    t.integer "lines_added"
    t.integer "lines_deleted"
    t.string "machine"
    t.jsonb "metadata", default: {}
    t.string "operating_system"
    t.string "project_id"
    t.float "time", null: false
    t.integer "tokens_used"
    t.datetime "updated_at", null: false
    t.string "user_id", null: false
    t.index ["project_id"], name: "index_heartbeat_events_on_project_id"
    t.index ["user_id", "agent_type"], name: "index_heartbeat_events_on_user_id_and_agent_type"
    t.index ["user_id", "language"], name: "index_heartbeat_events_on_user_id_and_language"
    t.index ["user_id", "time"], name: "index_heartbeat_events_on_user_id_and_time"
    t.index ["user_id"], name: "index_heartbeat_events_on_user_id"
  end

  create_table "projects", id: :string, force: :cascade do |t|
    t.string "color"
    t.datetime "created_at", null: false
    t.text "description"
    t.string "name", null: false
    t.datetime "updated_at", null: false
    t.string "user_id", null: false
    t.index ["user_id", "name"], name: "index_projects_on_user_id_and_name", unique: true
    t.index ["user_id"], name: "index_projects_on_user_id"
  end

  create_table "users", id: :string, force: :cascade do |t|
    t.datetime "created_at", null: false
    t.string "email", default: "", null: false
    t.string "encrypted_password", default: "", null: false
    t.string "name"
    t.datetime "remember_created_at"
    t.datetime "reset_password_sent_at"
    t.string "reset_password_token"
    t.string "timezone", default: "UTC"
    t.string "tinybird_token"
    t.datetime "updated_at", null: false
    t.index ["email"], name: "index_users_on_email", unique: true
    t.index ["reset_password_token"], name: "index_users_on_reset_password_token", unique: true
  end

  add_foreign_key "api_keys", "users"
  add_foreign_key "heartbeat_events", "projects"
  add_foreign_key "heartbeat_events", "users"
  add_foreign_key "projects", "users"
end
