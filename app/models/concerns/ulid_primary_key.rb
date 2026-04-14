module UlidPrimaryKey
  extend ActiveSupport::Concern

  included do
    self.primary_key = "id"

    before_create :set_ulid_id

    private

    def set_ulid_id
      self.id ||= ULID.generate.to_s
    end
  end
end
