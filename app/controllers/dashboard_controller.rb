class DashboardController < ApplicationController
  def index
    @stats = {
      total_events_today: today_events.count,
      coding_time_today: calculate_coding_time(today_events.order(time: :asc).to_a),
      active_projects: current_user.projects.count,
      total_tokens: today_events.sum(:tokens_used).to_i
    }

    @recent_heartbeats = current_user.heartbeat_events
                                      .order(time: :desc)
                                      .limit(20)
                                      .includes(:project)

    @language_breakdown = current_user.heartbeat_events
                                       .for_period(7.days.ago, Time.now)
                                       .group(:language)
                                       .count
                                       .reject { |k, _| k.blank? }
                                       .sort_by { |_, v| -v }
                                       .first(8)

    @agent_breakdown = current_user.heartbeat_events
                                    .for_period(7.days.ago, Time.now)
                                    .group(:agent_type)
                                    .count
                                    .sort_by { |_, v| -v }

    @daily_activity = build_daily_activity
  end

  private

  def today_events
    @today_events ||= current_user.heartbeat_events
                                   .for_period(Time.zone.now.beginning_of_day, Time.zone.now.end_of_day)
  end

  def calculate_coding_time(events)
    return 0 if events.empty?

    total = 0
    events.each_cons(2) do |a, b|
      diff = b.time - a.time
      total += diff if diff <= 120
    end
    total.to_i
  end

  def build_daily_activity
    (6.days.ago.to_date..Date.today).map do |date|
      start_ts = date.beginning_of_day.to_time.to_f
      end_ts = date.end_of_day.to_time.to_f
      events = current_user.heartbeat_events
                            .where("time >= ? AND time <= ?", start_ts, end_ts)
                            .order(time: :asc)
                            .to_a
      {
        date: date.iso8601,
        label: date.strftime("%a"),
        seconds: calculate_coding_time(events),
        events: events.count
      }
    end
  end

  def format_duration(seconds)
    hours = seconds / 3600
    mins = (seconds % 3600) / 60
    return "0 min" if hours == 0 && mins == 0
    parts = []
    parts << "#{hours}h" if hours > 0
    parts << "#{mins}m" if mins > 0
    parts.join(" ")
  end
  helper_method :format_duration
end
