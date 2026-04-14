module Billing
  class StripeConfiguration
    DEFAULT_PERSONAL_MONTHLY_PRICE_CENTS = 2_000

    class << self
      def secret_key
        ENV["STRIPE_SECRET_KEY"]
      end

      def publishable_key
        ENV["STRIPE_PUBLISHABLE_KEY"]
      end

      def webhook_secret
        ENV["STRIPE_WEBHOOK_SECRET"]
      end

      def personal_monthly_price_id
        ENV["STRIPE_PERSONAL_MONTHLY_PRICE_ID"]
      end

      def personal_monthly_price_label
        ENV["STRIPE_PERSONAL_MONTHLY_PRICE_LABEL"].presence || format("$%.2f/month", personal_monthly_price_cents / 100.0)
      end

      def personal_monthly_price_cents
        Integer(ENV.fetch("STRIPE_PERSONAL_MONTHLY_PRICE_CENTS", DEFAULT_PERSONAL_MONTHLY_PRICE_CENTS), exception: false) || DEFAULT_PERSONAL_MONTHLY_PRICE_CENTS
      end

      def portal_ready?
        secret_key.present?
      end

      def ready?
        portal_ready? && personal_monthly_price_id.present? && webhook_secret.present?
      end

      def checkout_ready?
        portal_ready? && personal_monthly_price_id.present?
      end
    end
  end
end
