export interface User {
	id: number;
	email: string;
	name: string;
	avatar_url?: string;
}

export type TripStatus = 'planned' | 'cancelled';

export interface Trip {
	id: number;
	owner_id: number;
	org_id?: number;
	cruise_id?: number;
	name: string;
	status: TripStatus;
	embark_date?: string;
	disembark_date?: string;
	countries?: string;
	start_port?: string;
	end_port?: string;
	captain_name?: string;
	yacht_id?: number;
	cost_total?: number;
	cost_per_person?: number;
	max_crew?: number;
	image_logo_url?: string;
	image_photo_url?: string;
	image_route_url?: string;
	description?: string;
	enroll_token?: string;
	created_at: string;
	updated_at: string;
}

export interface Voyage {
	id: number;
	owner_id: number;
	org_id?: number;
	cruise_id?: number;
	name: string;
	year?: number;
	embark_date?: string;
	disembark_date?: string;
	countries?: string;
	start_port?: string;
	end_port?: string;
	captain_name?: string;
	yacht_id?: number;
	hours_total: number;
	hours_sail: number;
	hours_engine: number;
	hours_over_6bf: number;
	miles: number;
	days: number;
	tidal_waters: number;
	cost_total?: number;
	cost_per_person?: number;
	image_logo_url?: string;
	image_photo_url?: string;
	image_route_url?: string;
	description?: string;
	created_at: string;
	updated_at: string;
}

export interface CompleteTripPayload {
	year?: number;
	hours_total?: number;
	hours_sail?: number;
	hours_engine?: number;
	hours_over_6bf?: number;
	miles?: number;
	days?: number;
	tidal_waters?: number;
}

export interface Yacht {
	id: number;
	owner_id: number;
	name: string;
	registration_no?: string;
	yacht_type?: string;
}

export interface CrewMember {
	id: number;
	owner_id: number;
	user_id?: number;
	full_name: string;
	email?: string;
	patent_number?: string;
}

export interface CrewAssignment {
	id: number;
	trip_id?: number;
	voyage_id?: number;
	crew_member_id: number;
	role: string;
	patent_number?: string;
	full_name: string;
	email?: string;
}

export interface Training {
	id: number;
	user_id: number;
	date?: string;
	name: string;
	organizer?: string;
	cost?: number;
	url?: string;
}

export interface DashboardStats {
	voyage_count: number;
	total_hours: number;
	total_miles: number;
	total_days: number;
	total_hours_sail: number;
	total_hours_engine: number;
	by_year: YearStats[];
}

export interface YearStats {
	year: number;
	voyage_count: number;
	total_hours: number;
	total_miles: number;
	total_days: number;
}

export interface UploadResponse {
	url: string;
}

export interface TripEnrollment {
	id: number;
	trip_id: number;
	user_id: number;
	note?: string;
	status: string;
	created_at: string;
	updated_at: string;
	user_name?: string;
	user_email?: string;
}

export interface PublicTrip {
	id: number;
	name: string;
	embark_date?: string;
	disembark_date?: string;
	countries?: string;
	start_port?: string;
	end_port?: string;
	description?: string;
	max_crew?: number;
	captain_name?: string;
	image_photo_url?: string;
}

export interface Cruise {
	id: number;
	org_id: number;
	name: string;
	embark_date?: string;
	disembark_date?: string;
	countries?: string;
	start_port?: string;
	end_port?: string;
	description?: string;
	image_logo_url?: string;
	image_photo_url?: string;
	image_route_url?: string;
	max_crew?: number;
	cost_per_person?: number;
	enroll_token?: string;
	created_at: string;
	updated_at: string;
}

export interface PublicCruise {
	id: number;
	org_id: number;
	name: string;
	embark_date?: string;
	disembark_date?: string;
	countries?: string;
	start_port?: string;
	end_port?: string;
	description?: string;
	image_photo_url?: string;
	max_crew?: number;
	cost_per_person?: number;
}

export interface CruiseEnrollment {
	id: number;
	cruise_id: number;
	user_id: number;
	trip_id?: number;
	note?: string;
	status: string;
	created_at: string;
	updated_at: string;
	user_name?: string;
	user_email?: string;
	trip_name?: string;
}

export type EnrollPageData =
	| {
			kind: 'trip';
			trip: PublicTrip;
			enrolled: boolean;
			enrollment?: TripEnrollment;
			accepted_count: number;
			total_count: number;
	  }
	| {
			kind: 'cruise';
			cruise: PublicCruise;
			trips: PublicTrip[];
			enrolled: boolean;
			enrollment?: CruiseEnrollment;
			accepted_count: number;
			total_count: number;
	  };

export interface VoyageOpinion {
	id: number;
	voyage_id: number;
	crew_member_id: number;
	file_path: string;
	file_format: string;
	full_name: string;
	created_at: string;
}

export interface Organization {
	id: number;
	name: string;
	slug: string;
	description?: string;
	logo_url?: string;
	pzz_club_number?: string;
	city?: string;
	website?: string;
	role?: string;
	created_at: string;
	updated_at: string;
}

export interface OrgMember {
	id: number;
	org_id: number;
	user_id: number;
	role: string;
	joined_at: string;
	user_name: string;
	user_email: string;
	user_avatar_url?: string;
}

export interface OrgInvite {
	id: number;
	org_id: number;
	token: string;
	role: string;
	created_by: number;
	expires_at?: string;
	max_uses?: number;
	use_count: number;
	created_at: string;
	creator_name?: string;
}

export interface OrgInviteInfo {
	org_name: string;
	org_slug: string;
	role: string;
	already_member: boolean;
}

export interface OrgDashboardStats {
	voyage_count: number;
	total_hours: number;
	total_miles: number;
	total_days: number;
	total_hours_sail: number;
	total_hours_engine: number;
	member_count: number;
	yacht_count: number;
	by_year: YearStats[];
}
