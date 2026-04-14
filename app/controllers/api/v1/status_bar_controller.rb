module Api
  module V1
    class StatusBarController < BaseController
      # GET /api/v1/users/current/statusbar/today
      def today
        start_of_day = Time.zone.now.beginning_of_day
        end_of_day = Time.zone.now.end_of_day

        events = current_user.heartbeat_events
                              .for_period(start_of_day, end_of_day)
                              .order(time: :asc)
                              .to_a

        total_seconds = calculate_coding_time(events)

        render json: {
          data: {
            grand_total: {
              decimal: (total_seconds / 3600.0).round(2),
              digital: format_digital(total_seconds),
              hours: total_seconds / 3600,
              minutes: (total_seconds % 3600) / 60,
              text: format_text(total_seconds),
              total_seconds: total_seconds
            }
          }
        }
      end

      private

      def calculate_coding_time(events)
        return 0 if events.empty?

        total = 0
        events.each_cons(2) do |a, b|
          diff = b.time - a.time
          total += diff if diff <= 120
        end
        total.to_i
      end

      def format_digital(seconds)
        hours = seconds / 3600
        mins = (seconds % 3600) / 60
        format("%d:%02d", hours, mins)
      end

      def format_text(seconds)
        hours = seconds / 3600
        mins = (seconds % 3600) / 60
        parts = []
        parts << "#{hours} hr" if hours > 0
        parts << "#{mins} min" if mins > 0
        parts.empty? ? "0 secs" : parts.join(" ")
      end
    end
  end
end
