class CreateHeartbeatEvents < ActiveRecord::Migration[8.1]
  def change
    create_table :heartbeat_events, id: false do |t|
      t.string :id, primary_key: true, null: false
      t.references :user, null: false, foreign_key: true, type: :string
      t.references :project, foreign_key: true, type: :string
      t.string :agent_type, null: false
      t.string :entity
      t.string :entity_type
      t.string :language
      t.string :branch
      t.string :operating_system
      t.string :machine
      t.float :time, null: false
      t.integer :lines_added
      t.integer :lines_deleted
      t.integer :tokens_used
      t.decimal :cost_usd, precision: 10, scale: 8
      t.boolean :is_write, default: false
      t.json :metadata, default: {}

      t.timestamps
    end

    add_index :heartbeat_events, [ :user_id, :time ]
    add_index :heartbeat_events, [ :user_id, :agent_type ]
    add_index :heartbeat_events, [ :user_id, :language ]
  end
end
