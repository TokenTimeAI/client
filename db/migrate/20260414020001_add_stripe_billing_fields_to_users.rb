class AddStripeBillingFieldsToUsers < ActiveRecord::Migration[8.1]
  def change
    add_column :users, :stripe_customer_id, :string
    add_column :users, :stripe_subscription_id, :string
    add_column :users, :stripe_subscription_status, :string
    add_column :users, :stripe_price_id, :string
    add_column :users, :subscription_current_period_end, :datetime
    add_column :users, :subscription_cancel_at_period_end, :boolean, default: false, null: false

    add_index :users, :stripe_customer_id, unique: true
    add_index :users, :stripe_subscription_id, unique: true
  end
end
