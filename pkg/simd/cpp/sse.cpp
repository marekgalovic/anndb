#include <immintrin.h>

inline
void _sum_vector(__m128 vec, float *result) {
	*result = (((float*)&vec)[0] + ((float*)&vec)[1] + ((float*)&vec)[2] + ((float*)&vec)[3]);
}

void euclidean_distance_squared(size_t len, float *a, float *b, float *result) {
	__m128 v1, v2;
	__m128 resVec = _mm_setzero_ps();
	for (int i = 0; i < len; i += 4) {
		v1 = _mm_load_ps(&a[i]);
		v2 = _mm_load_ps(&b[i]);
		v1 = _mm_sub_ps(v1, v2);
		v2 = _mm_mul_ps(v1, v1);
		resVec = _mm_add_ps(resVec, v2);
	}

	_sum_vector(resVec, result);
}

void manhattan_distance(size_t len, float *a, float *b, float *result) {
	__m128 v1, v2;
	__m128 resVec = _mm_setzero_ps();
	for (int i = 0; i < len; i += 4) {
		v1 = _mm_load_ps(&a[i]);
		v2 = _mm_load_ps(&b[i]);
		v1 = _mm_sub_ps(v1, v2);
		v2 = _mm_sqrt_ps(_mm_mul_ps(v1, v1));
		resVec = _mm_add_ps(resVec, v2);
	}

	_sum_vector(resVec, result);
}

void cosine_similarity_dot_norm(size_t len, float *a, float *b, float *result_dot, float *result_norm_squared) {
	__m128 v1, v2, dot, norm_a, norm_b;
	float norm_a_sum, norm_b_sum;
	dot = _mm_setzero_ps();
	norm_a = _mm_setzero_ps();
	norm_b = _mm_setzero_ps();
	for (int i = 0; i < len; i += 4) {
		v1 = _mm_load_ps(&a[i]);
		v2 = _mm_load_ps(&b[i]);
		dot = _mm_add_ps(dot, _mm_mul_ps(v1, v2));
		norm_a = _mm_add_ps(norm_a, _mm_mul_ps(v1, v1));
		norm_b = _mm_add_ps(norm_b, _mm_mul_ps(v2, v2));
	}

	_sum_vector(dot, result_dot);
	_sum_vector(norm_a, &norm_a_sum);
	_sum_vector(norm_b, &norm_b_sum);
	*result_norm_squared = (norm_a_sum * norm_b_sum);
}