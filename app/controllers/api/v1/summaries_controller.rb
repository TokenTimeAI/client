module Api
  module V1
    class SummariesController < BaseController
      # GET /api/v1/summaries?start=2024-01-01&end=2024-01-07&project=myproject
      def index
        start_time = parse_date(params[:start]) || 7.days.ago
        end_time = parse_date(params[:end])&.end_of_day || Time.now

        scope = current_user.heartbeat_events.for_period(start_time, end_time)
        scope = scope.by_agent(params[:agent]) if params[:agent].present?

        # Group by day and aggregate
        summaries = build_summaries(scope, start_time, end_time)

        render json: {
          data: summaries,
          start: start_time.iso8601,
          end: end_time.iso8601,
          cumulative_total: {
            seconds: summaries.sum { |s| s[:grand_total][:total_seconds] },
            decimal: summaries.sum { |s| s[:grand_total][:decimal] }.round(2),
            digital: format_digital(summaries.sum { |s| s[:grand_total][:total_seconds] }),
            text: format_text(summaries.sum { |s| s[:grand_total][:total_seconds] })
          }
        }
      end

      private

      def parse_date(str)
        return nil if str.blank?
        Time.zone.parse(str)
      rescue ArgumentError
        nil
      end

      def build_summaries(scope, start_time, end_time)
        events = scope.order(time: :asc).to_a

        # Group events by day
        by_day = events.group_by { |e| Time.at(e.time).utc.to_date }

        ((start_time.to_date)..(end_time.to_date)).map do |date|
          day_events = by_day[date] || []
          total_seconds = calculate_coding_time(day_events)

          {
            grand_total: {
              decimal: (total_seconds / 3600.0).round(2),
              digital: format_digital(total_seconds),
              hours: total_seconds / 3600,
              minutes: (total_seconds % 3600) / 60,
              text: format_text(total_seconds),
              total_seconds: total_seconds
            },
            categories: build_categories(day_events),
            projects: build_projects_summary(day_events),
            languages: build_languages_summary(day_events),
            editors: build_editors_summary(day_events),
            operating_systems: build_os_summary(day_events),
            range: {
              date: date.iso8601,
              start: date.beginning_of_day.iso8601,
              end: date.end_of_day.iso8601,
              text: date.strftime("%B %-d, %Y"),
              timezone: "UTC"
            }
          }
        end
      end

      def calculate_coding_time(events)
        return 0 if events.empty?

        total = 0
        sorted = events.sort_by(&:time)

        sorted.each_cons(2) do |a, b|
          diff = b.time - a.time
          total += diff if diff <= 120 # 2-minute heartbeat threshold
        end

        total.to_i
      end

      def build_categories(events)
        by_type = events.group_by(&:entity_type)
        by_type.map do |type, evts|
          seconds = calculate_coding_time(evts)
          {
            name: type&.capitalize || "Coding",
            total_seconds: seconds,
            digital: format_digital(seconds),
            decimal: (seconds / 3600.0).round(2),
            text: format_text(seconds),
            percent: 0
          }
        end
      end

      def build_projects_summary(events)
        by_project = events.group_by { |e| e.project&.name || "Unknown" }
        build_summary_items(by_project)
      end

      def build_languages_summary(events)
        by_language = events.group_by { |e| e.language || "Unknown" }
        build_summary_items(by_language)
      end

      def build_editors_summary(events)
        by_agent = events.group_by { |e| e.agent_type || "Unknown" }
        build_summary_items(by_agent)
      end

      def build_os_summary(events)
        by_os = events.group_by { |e| e.operating_system || "Unknown" }
        build_summary_items(by_os)
      end

      def build_summary_items(grouped)
        total_seconds = grouped.sum { |_, evts| calculate_coding_time(evts) }

        grouped.map do |name, evts|
          seconds = calculate_coding_time(evts)
          {
            name: name,
            total_seconds: seconds,
            digital: format_digital(seconds),
            decimal: (seconds / 3600.0).round(2),
            text: format_text(seconds),
            percent: total_seconds > 0 ? ((seconds.to_f / total_seconds) * 100).round(2) : 0
          }
        end.sort_by { |item| -item[:total_seconds] }
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
