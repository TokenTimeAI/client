class CreateApiKeys < ActiveRecord::Migration[8.1]
  def change
    create_table :api_keys, id: false do |t|
      t.string :id, primary_key: true, null: false
      t.references :user, null: false, foreign_key: true, type: :string
      t.string :name, null: false
      t.string :key, null: false
      t.datetime :last_used_at
      t.boolean :active, default: true, null: false

      t.timestamps
    end

    add_index :api_keys, :key, unique: true
  end
end
