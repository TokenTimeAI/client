module Api
  class BaseController < ActionController::API
    before_action :authenticate_api_key!

    private

    def authenticate_api_key!
      token = extract_token
      @current_user = ApiKey.authenticate(token)

      unless @current_user
        render json: { error: "Unauthorized" }, status: :unauthorized
      end
    end

    def current_user
      @current_user
    end

    def extract_token
      # Support Bearer token and Basic auth (WakaTime-style API key as username)
      auth_header = request.headers["Authorization"]
      return nil unless auth_header

      if auth_header.start_with?("Bearer ")
        auth_header.sub("Bearer ", "")
      elsif auth_header.start_with?("Basic ")
        decoded = Base64.decode64(auth_header.sub("Basic ", ""))
        # WakaTime uses the API key as the username in Basic auth
        decoded.split(":").first
      end
    end
  end
end
