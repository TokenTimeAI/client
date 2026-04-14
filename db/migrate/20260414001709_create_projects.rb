class CreateProjects < ActiveRecord::Migration[8.1]
  def change
    create_table :projects, id: false do |t|
      t.string :id, primary_key: true, null: false
      t.references :user, null: false, foreign_key: true, type: :string
      t.string :name, null: false
      t.string :color
      t.text :description

      t.timestamps
    end

    add_index :projects, [ :user_id, :name ], unique: true
  end
end
