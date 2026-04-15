class AddSessionTimingDimensionsToHeartbeatEvents < ActiveRecord::Migration[8.1]
  def change
    change_table :heartbeat_events, bulk: true do |t|
      t.datetime :session_started_at
      t.datetime :session_ended_at
      t.integer :session_duration_seconds
      t.integer :agent_active_seconds
      t.integer :human_active_seconds
      t.integer :idle_seconds
    end

    add_index :heartbeat_events, [ :user_id, :session_started_at ]
    add_index :heartbeat_events, [ :user_id, :session_ended_at ]
  end
end
