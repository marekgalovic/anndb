	.section	__TEXT,__text,regular,pure_instructions
	.build_version macos, 10, 15	sdk_version 10, 15
	.intel_syntax noprefix
	.globl	__Z26euclidean_distance_squaredmPfS_S_ ## -- Begin function _Z26euclidean_distance_squaredmPfS_S_
	.p2align	4, 0x90
__Z26euclidean_distance_squaredmPfS_S_: ## @_Z26euclidean_distance_squaredmPfS_S_
## %bb.0:
	push	rbp
	mov	rbp, rsp
	and	rsp, -8
	test	rdi, rdi
	je	LBB0_1
## %bb.2:
	dec	rdi
	mov	r9, rdi
	shr	r9, 2
	lea	r8d, [r9 + 1]
	and	r8d, 3
	cmp	rdi, 12
	jae	LBB0_8
## %bb.3:
	xorps	xmm0, xmm0
	xor	edi, edi
	test	r8, r8
	jne	LBB0_5
	jmp	LBB0_7
LBB0_1:
	xorps	xmm0, xmm0
	jmp	LBB0_7
LBB0_8:
	lea	rax, [r8 - 1]
	sub	rax, r9
	xorps	xmm1, xmm1
	xor	edi, edi
	.p2align	4, 0x90
LBB0_9:                                 ## =>This Inner Loop Header: Depth=1
	movaps	xmm2, xmmword ptr [rsi + 4*rdi]
	movaps	xmm3, xmmword ptr [rsi + 4*rdi + 16]
	movaps	xmm4, xmmword ptr [rsi + 4*rdi + 32]
	movaps	xmm0, xmmword ptr [rsi + 4*rdi + 48]
	subps	xmm2, xmmword ptr [rdx + 4*rdi]
	mulps	xmm2, xmm2
	addps	xmm2, xmm1
	subps	xmm3, xmmword ptr [rdx + 4*rdi + 16]
	mulps	xmm3, xmm3
	addps	xmm3, xmm2
	subps	xmm4, xmmword ptr [rdx + 4*rdi + 32]
	mulps	xmm4, xmm4
	addps	xmm4, xmm3
	subps	xmm0, xmmword ptr [rdx + 4*rdi + 48]
	mulps	xmm0, xmm0
	addps	xmm0, xmm4
	add	rdi, 16
	movaps	xmm1, xmm0
	add	rax, 4
	jne	LBB0_9
## %bb.4:
	test	r8, r8
	je	LBB0_7
LBB0_5:
	shl	rdi, 2
	neg	r8
	.p2align	4, 0x90
LBB0_6:                                 ## =>This Inner Loop Header: Depth=1
	movaps	xmm1, xmmword ptr [rsi + rdi]
	subps	xmm1, xmmword ptr [rdx + rdi]
	mulps	xmm1, xmm1
	addps	xmm0, xmm1
	add	rdi, 16
	inc	r8
	jne	LBB0_6
LBB0_7:
	movshdup	xmm1, xmm0      ## xmm1 = xmm0[1,1,3,3]
	addss	xmm1, xmm0
	movaps	xmm2, xmm0
	unpckhpd	xmm2, xmm0      ## xmm2 = xmm2[1],xmm0[1]
	addss	xmm2, xmm1
	shufps	xmm0, xmm0, 231         ## xmm0 = xmm0[3,1,2,3]
	addss	xmm0, xmm2
	movss	dword ptr [rcx], xmm0
	mov	rsp, rbp
	pop	rbp
	ret
                                        ## -- End function
	.globl	__Z18manhattan_distancemPfS_S_ ## -- Begin function _Z18manhattan_distancemPfS_S_
	.p2align	4, 0x90
__Z18manhattan_distancemPfS_S_:         ## @_Z18manhattan_distancemPfS_S_
## %bb.0:
	push	rbp
	mov	rbp, rsp
	and	rsp, -8
	test	rdi, rdi
	je	LBB1_1
## %bb.2:
	dec	rdi
	shr	rdi, 2
	lea	r8d, [rdi + 1]
	and	r8d, 1
	test	rdi, rdi
	je	LBB1_3
## %bb.7:
	lea	rax, [r8 - 1]
	sub	rax, rdi
	xorps	xmm0, xmm0
	xor	edi, edi
	.p2align	4, 0x90
LBB1_8:                                 ## =>This Inner Loop Header: Depth=1
	movaps	xmm1, xmmword ptr [rsi + 4*rdi]
	movaps	xmm2, xmmword ptr [rsi + 4*rdi + 16]
	subps	xmm1, xmmword ptr [rdx + 4*rdi]
	mulps	xmm1, xmm1
	sqrtps	xmm1, xmm1
	addps	xmm1, xmm0
	subps	xmm2, xmmword ptr [rdx + 4*rdi + 16]
	mulps	xmm2, xmm2
	sqrtps	xmm0, xmm2
	addps	xmm0, xmm1
	add	rdi, 8
	add	rax, 2
	jne	LBB1_8
## %bb.4:
	test	r8, r8
	jne	LBB1_5
	jmp	LBB1_6
LBB1_1:
	xorps	xmm0, xmm0
	jmp	LBB1_6
LBB1_3:
	xorps	xmm0, xmm0
	xor	edi, edi
	test	r8, r8
	je	LBB1_6
LBB1_5:
	movaps	xmm1, xmmword ptr [rsi + 4*rdi]
	subps	xmm1, xmmword ptr [rdx + 4*rdi]
	mulps	xmm1, xmm1
	sqrtps	xmm1, xmm1
	addps	xmm0, xmm1
LBB1_6:
	movshdup	xmm1, xmm0      ## xmm1 = xmm0[1,1,3,3]
	addss	xmm1, xmm0
	movaps	xmm2, xmm0
	unpckhpd	xmm2, xmm0      ## xmm2 = xmm2[1],xmm0[1]
	addss	xmm2, xmm1
	shufps	xmm0, xmm0, 231         ## xmm0 = xmm0[3,1,2,3]
	addss	xmm0, xmm2
	movss	dword ptr [rcx], xmm0
	mov	rsp, rbp
	pop	rbp
	ret
                                        ## -- End function
	.globl	__Z26cosine_similarity_dot_normmPfS_S_S_ ## -- Begin function _Z26cosine_similarity_dot_normmPfS_S_S_
	.p2align	4, 0x90
__Z26cosine_similarity_dot_normmPfS_S_S_: ## @_Z26cosine_similarity_dot_normmPfS_S_S_
## %bb.0:
	push	rbp
	mov	rbp, rsp
	and	rsp, -8
	test	rdi, rdi
	je	LBB2_1
## %bb.2:
	dec	rdi
	shr	rdi, 2
	lea	r9d, [rdi + 1]
	and	r9d, 1
	test	rdi, rdi
	je	LBB2_3
## %bb.7:
	lea	rax, [r9 - 1]
	sub	rax, rdi
	xorps	xmm3, xmm3
	xor	edi, edi
	xorps	xmm4, xmm4
	xorps	xmm2, xmm2
	.p2align	4, 0x90
LBB2_8:                                 ## =>This Inner Loop Header: Depth=1
	movaps	xmm5, xmmword ptr [rsi + 4*rdi]
	movaps	xmm1, xmmword ptr [rsi + 4*rdi + 16]
	movaps	xmm6, xmmword ptr [rdx + 4*rdi]
	movaps	xmm0, xmmword ptr [rdx + 4*rdi + 16]
	movaps	xmm7, xmm5
	mulps	xmm7, xmm6
	addps	xmm7, xmm2
	mulps	xmm5, xmm5
	addps	xmm5, xmm4
	mulps	xmm6, xmm6
	addps	xmm6, xmm3
	movaps	xmm2, xmm1
	mulps	xmm2, xmm0
	addps	xmm2, xmm7
	mulps	xmm1, xmm1
	addps	xmm1, xmm5
	mulps	xmm0, xmm0
	addps	xmm0, xmm6
	add	rdi, 8
	movaps	xmm3, xmm0
	movaps	xmm4, xmm1
	add	rax, 2
	jne	LBB2_8
## %bb.4:
	test	r9, r9
	jne	LBB2_5
	jmp	LBB2_6
LBB2_1:
	xorps	xmm2, xmm2
	xorps	xmm1, xmm1
	xorps	xmm0, xmm0
	jmp	LBB2_6
LBB2_3:
	xorps	xmm0, xmm0
	xor	edi, edi
	xorps	xmm1, xmm1
	xorps	xmm2, xmm2
	test	r9, r9
	je	LBB2_6
LBB2_5:
	movaps	xmm3, xmmword ptr [rsi + 4*rdi]
	movaps	xmm4, xmmword ptr [rdx + 4*rdi]
	movaps	xmm5, xmm3
	mulps	xmm3, xmm4
	mulps	xmm4, xmm4
	addps	xmm0, xmm4
	mulps	xmm5, xmm5
	addps	xmm1, xmm5
	addps	xmm2, xmm3
LBB2_6:
	movshdup	xmm3, xmm2      ## xmm3 = xmm2[1,1,3,3]
	addss	xmm3, xmm2
	movaps	xmm4, xmm2
	unpckhpd	xmm4, xmm2      ## xmm4 = xmm4[1],xmm2[1]
	addss	xmm4, xmm3
	shufps	xmm2, xmm2, 231         ## xmm2 = xmm2[3,1,2,3]
	addss	xmm2, xmm4
	movss	dword ptr [rcx], xmm2
	movshdup	xmm2, xmm1      ## xmm2 = xmm1[1,1,3,3]
	addss	xmm2, xmm1
	movaps	xmm3, xmm1
	unpckhpd	xmm3, xmm1      ## xmm3 = xmm3[1],xmm1[1]
	addss	xmm3, xmm2
	shufps	xmm1, xmm1, 231         ## xmm1 = xmm1[3,1,2,3]
	addss	xmm1, xmm3
	movshdup	xmm2, xmm0      ## xmm2 = xmm0[1,1,3,3]
	addss	xmm2, xmm0
	movaps	xmm3, xmm0
	unpckhpd	xmm3, xmm0      ## xmm3 = xmm3[1],xmm0[1]
	addss	xmm3, xmm2
	shufps	xmm0, xmm0, 231         ## xmm0 = xmm0[3,1,2,3]
	addss	xmm0, xmm3
	mulss	xmm0, xmm1
	movss	dword ptr [r8], xmm0
	mov	rsp, rbp
	pop	rbp
	ret
                                        ## -- End function

.subsections_via_symbols
