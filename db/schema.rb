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

ActiveRecord::Schema[8.1].define(version: 2026_04_15_061000) do
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

  create_table "device_authorizations", id: :string, force: :cascade do |t|
    t.string "api_key_id"
    t.datetime "approved_at"
    t.datetime "claimed_at"
    t.datetime "created_at", null: false
    t.string "device_code_digest", null: false
    t.datetime "expires_at", null: false
    t.integer "interval", default: 5, null: false
    t.string "machine_name"
    t.string "status", default: "pending", null: false
    t.datetime "updated_at", null: false
    t.string "user_code", null: false
    t.string "user_id"
    t.index ["api_key_id"], name: "index_device_authorizations_on_api_key_id"
    t.index ["device_code_digest"], name: "index_device_authorizations_on_device_code_digest", unique: true
    t.index ["expires_at"], name: "index_device_authorizations_on_expires_at"
    t.index ["status"], name: "index_device_authorizations_on_status"
    t.index ["user_code"], name: "index_device_authorizations_on_user_code", unique: true
    t.index ["user_id"], name: "index_device_authorizations_on_user_id"
  end

  create_table "heartbeat_events", id: :string, force: :cascade do |t|
    t.string "agent_type", null: false
    t.integer "agent_active_seconds"
    t.string "branch"
    t.decimal "cost_usd", precision: 10, scale: 8
    t.datetime "created_at", null: false
    t.string "entity"
    t.string "entity_type"
    t.datetime "session_ended_at"
    t.integer "human_active_seconds"
    t.integer "idle_seconds"
    t.boolean "is_write", default: false
    t.string "language"
    t.integer "lines_added"
    t.integer "lines_deleted"
    t.string "machine"
    t.jsonb "metadata", default: {}
    t.string "operating_system"
    t.string "project_id"
    t.datetime "session_started_at"
    t.integer "session_duration_seconds"
    t.float "time", null: false
    t.integer "tokens_used"
    t.datetime "updated_at", null: false
    t.string "user_id", null: false
    t.index ["project_id"], name: "index_heartbeat_events_on_project_id"
    t.index ["user_id", "agent_type"], name: "index_heartbeat_events_on_user_id_and_agent_type"
    t.index ["user_id", "session_ended_at"], name: "index_heartbeat_events_on_user_id_and_session_ended_at"
    t.index ["user_id", "session_started_at"], name: "index_heartbeat_events_on_user_id_and_session_started_at"
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
    t.string "provider"
    t.datetime "remember_created_at"
    t.datetime "reset_password_sent_at"
    t.string "reset_password_token"
    t.string "stripe_customer_id"
    t.string "stripe_price_id"
    t.string "stripe_subscription_id"
    t.string "stripe_subscription_status"
    t.boolean "subscription_cancel_at_period_end", default: false, null: false
    t.datetime "subscription_current_period_end"
    t.string "timezone", default: "UTC"
    t.string "tinybird_token"
    t.string "uid"
    t.datetime "updated_at", null: false
    t.index ["email"], name: "index_users_on_email", unique: true
    t.index ["provider", "uid"], name: "index_users_on_provider_and_uid", unique: true
    t.index ["reset_password_token"], name: "index_users_on_reset_password_token", unique: true
    t.index ["stripe_customer_id"], name: "index_users_on_stripe_customer_id", unique: true
    t.index ["stripe_subscription_id"], name: "index_users_on_stripe_subscription_id", unique: true
  end

  add_foreign_key "api_keys", "users"
  add_foreign_key "device_authorizations", "api_keys"
  add_foreign_key "device_authorizations", "users"
  add_foreign_key "heartbeat_events", "projects"
  add_foreign_key "heartbeat_events", "users"
  add_foreign_key "projects", "users"
end
