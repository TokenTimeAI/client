class User < ApplicationRecord
  # Include default devise modules. Others available are:
  # :confirmable, :lockable, :timeoutable, :trackable and :omniauthable
  devise :database_authenticatable, :registerable,
         :recoverable, :rememberable, :validatable

  has_many :api_keys, dependent: :destroy
  has_many :projects, dependent: :destroy
  has_many :heartbeat_events, dependent: :destroy

  validates :name, presence: true

  def display_name
    name.presence || email.split("@").first
  end

  def active_api_key
    api_keys.active.first
  end
end
