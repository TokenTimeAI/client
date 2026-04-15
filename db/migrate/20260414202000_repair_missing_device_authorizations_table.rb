class RepairMissingDeviceAuthorizationsTable < ActiveRecord::Migration[8.1]
  def change
    return if table_exists?(:device_authorizations)

    create_table :device_authorizations, id: false do |t|
      t.string :id, primary_key: true, null: false
      t.string :device_code_digest, null: false
      t.string :user_code, null: false
      t.string :machine_name
      t.string :status, null: false, default: "pending"
      t.datetime :expires_at, null: false
      t.integer :interval, null: false, default: 5
      t.references :user, null: true, foreign_key: true, type: :string
      t.references :api_key, null: true, foreign_key: true, type: :string
      t.datetime :approved_at
      t.datetime :claimed_at

      t.timestamps
    end

    add_index :device_authorizations, :device_code_digest, unique: true
    add_index :device_authorizations, :user_code, unique: true
    add_index :device_authorizations, :status
    add_index :device_authorizations, :expires_at
  end
end
