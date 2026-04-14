module Api
  module V1
    class DeviceAuthorizationsController < ActionController::API
      # POST /api/v1/device_authorizations
      def create
        authorization = DeviceAuthorization.create_pending!(machine_name: create_params[:machine_name])

        render json: {
          device_code: authorization.device_code,
          user_code: authorization.user_code,
          verification_uri: device_authorization_url(user_code: authorization.user_code),
          expires_in: [ (authorization.expires_at - Time.current).ceil, 0 ].max,
          interval: authorization.interval
        }, status: :created
      end

      # POST /api/v1/device_authorizations/poll
      def poll
        authorization = DeviceAuthorization.find_by_device_code(poll_params[:device_code])
        return render_oauth_error("invalid_device_code", :not_found) unless authorization

        payload = nil
        error = nil
        status = :bad_request

        authorization.with_lock do
          authorization.expire_if_needed!

          case authorization.status
          when "pending"
            error = "authorization_pending"
          when "approved"
            payload = authorization.claim!(api_base_url: "#{request.base_url}/api/v1")
            status = :ok
          when "claimed"
            error = "authorization_claimed"
            status = :gone
          when "expired"
            error = "expired_token"
          else
            error = "invalid_request"
            status = :unprocessable_entity
          end
        end

        if payload
          render json: payload, status: status
        else
          render_oauth_error(error, status)
        end
      end

      private

      def create_params
        params.permit(:machine_name)
      end

      def poll_params
        params.permit(:device_code)
      end

      def render_oauth_error(error, status)
        render json: { error: error }, status: status
      end
    end
  end
end
