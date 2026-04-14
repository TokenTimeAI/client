module Api
  module V1
    class UsersController < BaseController
      # GET /api/v1/users/current
      def current
        render json: {
          data: {
            id: current_user.id,
            email: current_user.email,
            name: current_user.display_name,
            display_name: current_user.display_name,
            full_name: current_user.name,
            timezone: current_user.timezone,
            created_at: current_user.created_at
          }
        }
      end
    end
  end
end
